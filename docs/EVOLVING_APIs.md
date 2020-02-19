# Dealing with evolving APIs of components under test

## Problem statement

Testground makes it simple to compile test plans against arbitrary versions of
upstream dependencies.

This feature is powerful in that it serves as the cornerstone of comparative
testing, enabling users to compare how version A of a specific module performs
against versions B, C, and D. And even to orchestrate scenarios where all those
versions are tested together within a single composition run.

Power does not come devoid of challenges. Concretely, the main challenge here is
sustainable and straightforward maintenance and evolution of the test plan and
the components under test. None of those are stationary in time. They are bound
to evolve both behaviourally and contractually (API changes).

Ensuring that a test plan remains compatible and buildable against all relevant
versions of upstream dependencies over time (or against a declared version range
thereof) isn't something that will happen automatically, nor trivially.

Testground needs to provide the plumbing for test plans to remain backwards and
forwards-compatible with upstream dependencies, and this proposal introduces
approaches to resolve this concern.

## Constraints

1. We want test plans to be _wildcard buildable_.
     * We categorically want to avoid maintaining forks of test plans, where
       each fork builds against a specific upstream dependency range.
     * Such a scenario would rapidly turn into a maintenance chaos, and would
       scale poorly when targeting multiple changing upstreams, turning into an
       exponential hairball, as we'd have to maintain forks for the cartesian
       product of all targeted upstreams.
2. This problem applies only to test plans written in statically-typed languages
   (e.g. Go).
     * Dynamically-typed languages do not suffer from this problem, as they can
       perform all kinds of tricks to do feature/API detection, and vary the
       test plan's behaviour in accordance.
3. How testground deals with this must refrain from using language-specific
   assumptions and semantics in its core.
     * Language-specific elements must be pushed to the edges, i.e. the
       appropriate builders, if relevant.

## Example situations

Given a `dht` test plan written in Go that exercises the `kad-dht` module in
various ways, this is a non-comprehensive list of situations we may be
confronted with as both items evolve.

Note that many of these problems would not appear in a dynamically-typed
language such as JavaScript or Python. Indeed, most of these issues emerge from
compiler type checking at build time.

1. `kad-dht/v4` introduces a new `Option`, which we wire to a test parameter
   (e.g. `BucketSize`, `PeerValidationFrequency`). Compiling the test plan
   against versions < v4 will fail.
2. `kad-dht/v4` changes the signature of an existing method. Adjusting the test
   plan will make it fail against < v4; not doing so will make it fail with v4+.
3. `kad-dht/v4` introduces a new field in a public struct, which our test plan
   wants to use. Building it against < v4 will now fail.
4. `kad-dht/v4` introduces a package-level function we want to call in our test
   plan. Building against < v4 will fail, because the function didn't exist back
   then.

## Potential mechanisms for Go test plans

We have identified the following building blocks/mechanisms as helpful
primitives to combine in designing a solution to deal with this problem domain. 

* Shim/stub objects.
* Build tags.
* Type assertions against interfaces.
* Hardcore reflection.

In this section we reflect about the applicability of each.

### Stub/shim objects

To bridge future/past API versions for backwards and forwards compatibility.

Ownership:

* **Owned by the evolving component:** discarded. Shim selection is not easy,
  forward-compatibility not feasible/practical. Will not be used by anybody but
  testground plans, so placing them here distances them too much from the only
  place where they'll be used.

* **Owned by the test plan:** preferred. Places the burden of maintenance on the
  codebase that's actually _needs_ these components, multiplexes over them, and
  the one that'll spot breakage immediately.

### Build tags

The builder can inject build tags representing upstream dependency versions,
such that test plans can then adequately select which shims to activate at build
time.
  
Each upstream dep translates to a set of build tags to facilitate
matching at the different levels of semver.

For example, dependency `github.com/libp2p/go-libp2p-kad-dht@0.3.2` would
translate to three build tags:

* `github.com/libp2p/go-libp2p-kad-dht@0`
* `github.com/libp2p/go-libp2p-kad-dht@0.3`
* `github.com/libp2p/go-libp2p-kad-dht@0.3.2`

Source files in test plans are free to match against any of these build tags.

This approach will naturally lead to accumulating (over time) a repository of
shims that get activated automatically when building against specific versions
of an upstream dependency, without the developer having to remember to specify
any manual flags.

Admittedly one blind spot of this solution lies in non-released versions, which
we import by referencing commit hashes. Semver matching is impossible in this
case, and the builder can only inline the actual commit hash in the build tag,
which means that the developer will need to alter the build tag header on the
shim sources every time they target a new commit. Unfortunately, I can't think
of a better solution for this case, but I also think it is not as cumbersome as
it first appears, because it's likely that the developer is experimenting
heavily and rebuilding frequently if they are hitting commit hashes.

Another matter to think about is how to share/reuse these shims across multiple
test plans. At first sight, it seems there's nothing complex about this, as we
can simply extract the shims to a dedicated repo, which all relevant test plans
exercising the shimmed component import.

Open questions:

* limitations on number/length/syntax/format of build tags that can be set on a
  build?
* test plan developer experience. Things like autocompletion, gopls, etc. may
  raise errors because they are not aware of the build tags that will
  effectively be applied during the build.

To keep the build tag namespace tidy and debloated, we can extend test plan
manifests with a field that enumerates upstream dependencies we want to inject
build tags for, via ant/regex patterns.

### Type assertions

Against single-method – potentially anonymous – interfaces, in order to perform
feature detection.

```go
dht, _ := kaddht.NewDHT()
if i1, ok := dht.(interface{
  MethodA(foo *kaddht.TypeA) (*kaddht.TypeB, error)
}); ok {
  // do something with i1.MethodA.
} else if i2, ok := dht.(interface{
  MethodB(foo *kaddht.TypeA) (*kaddht.TypeC, error)
}); ok {
  // do something with i2.MethodB.
}
```

This is useful in two scenarios:

1. A method signature has changed.
2. A method has been added/removed.

It is useless for, amongst others:

1. Package-level functions.
2. Newly-introduced types.
3. Deleted types.

As a result, we do not consider this approach generally feasible by itself, in
that drawbacks (2) and (3) actually make assumptions about available types.

However, this approach _could_ be combined with shims to layer on conditional
activation/selection based on upstream dependency versions.

### Reflection

Reflection in Go is great, but insufficient by itself in the advent of type
creation and removal. Go cannot load types dynamically (like Java can via the
ClassLoader), so in order to use reflection, one actually has to refer to the
type explicitly, which may disappear/change down the line.

However, like type assertions, this approach _could_ be combined with shims to
layer on conditional activation/selection based on upstream dependency versions.

## Proposed solution

The proposed solution for solving this problem in Go test plans revolves around
_conditional shim activation_. Testground itself will facilitate the contextual
activation of shims by injecting the appropriate go build tags into the build,
attending to a set of manifest-defined rules.

### Shimming example

Test plan developers dealing with evolving upstream APIs will need to
encapsulate changing behaviour behind shim objects.

To illustrate how this would work in practice, let's assume that upstream
`go-libp2p-kad-dht@v0.3.2` introduces a brand new
`opts.ReplacementQueueSize(uint)` option. This option was not present in
`go-libp2p-kad-dht@v0.3.1`, therefore the compilation would fail when targeting
that upstream version (and earlier).

The `dht` test plan imports this dependency, and adds a test plan option such
that the tester can inject a value for this API option.

Compiling the new test plan against `go-libp2p-kad-dht@v0.3.1` would fail, as
the `opts.ReplacementQueueSize(uint)` function hadn't been introduced yet.

Bridging this discrepancy via a shim could take the following form factor. Note
that the filenames are irrelevant and only illustrative (but they could be
reckoned a sensible pattern):

```go
// filename: opts_post_kaddht_v0.3.2.go
//
// +build shim-a
import kaddht "github.com/libp2p/go-libp2p-kad-dht"

func (so *SetupOpts) ToDHTOptions(_ *RunEnv) (res []kaddht.Option) {
  // ... parse other options ...
  if rqsize := so.ReplacementQueueSize; rqsize != nil {
    res = append(res, kaddht.ReplacementQueueSize(*reqsize))
  }
  return res
}
```

```go
// filename: opts_pre_kaddht_v0.3.2.go
//
// +build shim-b
import kaddht "github.com/libp2p/go-libp2p-kad-dht"

func (so *SetupOpts) ToDHTOptions(runenv *RunEnv) (res []kaddht.Option) {
  // ... parse other options ...
  if rqsize := so.ReplacementQueueSize; rqsize != nil {
    runenv.RecordMessage("ignoring replacement queue size option (value: %d); this version of go-libp2p-kad-dht does not support it", *rqsize)
  }
  return res
}
```



### Manual shim activation (P0)

When the developer is testing against unreleased upstream commits or branches,
it is unfeasible to apply automatic version selection.

Instead, the developer could specify a list of build tags or selectors they want
enabled, under the `group.build` section of each group of their composition
TOML.

```toml
[[groups]]
id = "bootstrappers"

  [groups.build]
  selectors = ["shim-a"]

[[groups]]
id = "clients"

  [groups.build]
  selectors = ["shim-b"]
  dependencies = [
    { module = "github.com/libp2p/go-libp2p-kad-dht", version = "995fee9e5345fdd7c151a5fe871252262db4e788"},
    { module = "github.com/libp2p/go-libp2p", version = "76944c4fc848530530f6be36fb22b70431ca506c"},
  ]
```

### Version-based shim activation (P1)

Version-based shim activation rules could be defined in the test plan manifest
TOML, under a `[[rules.selector]]` section. We are choosing to generify the term
"build tag" (Go specific), to the language-neutral term "selector", since it
accurately describes the intention within this context: to select source files
for the build.

> TODO: consider industry-standard version range specification formats, such as
> Maven or npm. Do not reinvent the wheel here. The below version ranges are
> placeholders merely for illustrative purposes.

```toml
[[rules.selectors]]
module = "github.com/libp2p/go-libp2p-kad-dht"
when = [
  { version = "[0,0.3.2)",  select = "shim-a" },
  { version = "[0.3.2,)",   select = "shim-b" },
]

[[rules.selectors]]
module = "github.com/libp2p/go-libp2p"
when = [
  { version = "[0,1)",  select = "shim-foo" },
  { version = "[1,)",   select = "shim-bar" },
]
```

#### Implementation notes

Testground builders are expected to calculate and return the effective
dependency graph of a build, under the `api.BuildOutput.Dependencies` field.

We'd need to calculate the dependency graph _prior_ to calling `go build .` and
intersect it with the selector rules, in order to compute which build tags to
apply when calling `go build .`.

The Go code to do so is seemingly straightforward, and could be called directly
from the `exec:go` builder. However, the `docker:go` builder performs the build
in a Docker build, and we want to avoid duplicating this logic.

A fair solution consists of wrapping this code in a main function, and
harnessing `go generate` to call it from within a Docker build. It would output
the build tags on stdout, which we'd then 
