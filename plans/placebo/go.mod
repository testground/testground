module github.com/ipfs/testground/plans/placebo

go 1.13

require (
	github.com/ipfs/testground/sdk/runtime v0.0.0-00010101000000-000000000000
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
)

replace github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
