# Copyright 2016 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load("@io_bazel_rules_go//go:def.bzl", "go_context", "go_rule")
load("@io_bazel_rules_go//go/private:providers.bzl", "GoArchive")
load("@io_bazel_rules_go//go/private:rules/prefix.bzl", "go_prefix_default")

def _compute_genrule_variables(resolved_srcs, resolved_outs):
  variables = {"SRCS": cmd_helper.join_paths(" ", resolved_srcs),
               "OUTS": cmd_helper.join_paths(" ", resolved_outs)}
  if len(resolved_srcs) == 1:
    variables["<"] = list(resolved_srcs)[0].path
  if len(resolved_outs) == 1:
    variables["@"] = list(resolved_outs)[0].path
  return variables

def _compute_genrule_command(ctx, go):
  workspace_root = '$$(pwd)'
  if ctx.build_file_path.startswith('external/'):
    # We want GO_WORKSPACE to point at the root directory of the Bazel
    # workspace containing this go_genrule's BUILD file. If it's being
    # included in a different workspace as an external dependency, the
    # link target must point to the external subtree instead of the main
    # workspace (which contains code we don't care about).
    #
    # Given a build file path like "external/foo/bar/BUILD", the following
    # slash split+join sets external_dep_prefix to "external/foo" and the
    # effective workspace root to "$PWD/external/foo/".
    external_dep_prefix = '/'.join(ctx.build_file_path.split('/')[:2])
    workspace_root = '$$(pwd)/' + external_dep_prefix

  cmd = [
      'set -e',
      'export GOROOT=$$(pwd)/' + go.root,
      'export GOOS=' + go.mode.goos,
      'export GOARCH=' + go.mode.goarch,
      # setup main GOPATH
      'GENRULE_TMPDIR=$$(mktemp -d $${TMPDIR:-/tmp}/bazel_%s_XXXXXXXX)' % ctx.attr.name,
      'export GOPATH=$${GENRULE_TMPDIR}/gopath',
      'export GO_WORKSPACE=$${GOPATH}/src/' + ctx.attr._go_prefix.go_prefix,
      'mkdir -p $${GO_WORKSPACE%/*}',
      'ln -s %s/ $${GO_WORKSPACE}' % (workspace_root,),
      'if [[ ! -e $${GO_WORKSPACE}/external ]]; then ln -s $$(pwd)/external/ $${GO_WORKSPACE}/; fi',
      'if [[ ! -e $${GO_WORKSPACE}/bazel-out ]]; then ln -s $$(pwd)/bazel-out/ $${GO_WORKSPACE}/; fi',
      # setup genfile GOPATH
      'export GENGOPATH=$${GENRULE_TMPDIR}/gengopath',
      'export GENGO_WORKSPACE=$${GENGOPATH}/src/' + ctx.attr._go_prefix.go_prefix,
      'mkdir -p $${GENGO_WORKSPACE%/*}',
      'ln -s $$(pwd)/$(GENDIR) $${GENGO_WORKSPACE}',
      # drop into WORKSPACE
      'export GOPATH=$${GOPATH}:$${GENGOPATH}',
      'cd $${GO_WORKSPACE}',
      # execute user command
      ctx.attr.cmd.strip(' \t\n\r'),
  ]
  return '\n'.join(cmd)

def _go_genrule_impl(ctx):
  go = go_context(ctx)

  all_srcs = depset(go.stdlib.files)
  label_dict = {}

  for dep in ctx.attr.go_deps:
    for archive in dep[GoArchive].transitive:
      all_srcs += archive.srcs

  for dep in ctx.attr.srcs:
    all_srcs += dep.files
    label_dict[dep.label] = dep.files

  cmd = _compute_genrule_command(ctx, go)

  resolved_inputs, argv, runfiles_manifests = ctx.resolve_command(
      command=cmd,
      attribute="cmd",
      expand_locations=True,
      make_variables=_compute_genrule_variables(all_srcs, depset(ctx.outputs.outs)),
      tools=ctx.attr.tools,
      label_dict=label_dict
  )

  ctx.action(
      inputs = list(all_srcs) + resolved_inputs,
      outputs = ctx.outputs.outs,
      env = ctx.configuration.default_shell_env,
      command = argv,
      progress_message = "%s %s" % (ctx.attr.message, ctx),
      mnemonic = "GoGenrule",
  )

# We have codegen procedures that depend on the "go/*" stdlib packages
# and thus depend on executing with a valid GOROOT and GOPATH containing
# some amount transitive go src of dependencies. This go_genrule enables
# the creation of these sandboxes.
go_genrule = go_rule(
    _go_genrule_impl,
    attrs = {
        "srcs": attr.label_list(allow_files = True),
        "tools": attr.label_list(
            cfg = "host",
            allow_files = True,
        ),
        "outs": attr.output_list(mandatory = True),
        "cmd": attr.string(mandatory = True),
        "go_deps": attr.label_list(),
        "importpath": attr.string(),
        "message": attr.string(),
        "executable": attr.bool(default = False),
        "_go_prefix": attr.label(default = go_prefix_default),
    },
    output_to_genfiles = True,
)
