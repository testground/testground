package build

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// CollapseOptionsMap converts a string map to a slice of key=value strings,
// canonicalising the result by sorting the map first.
func CollapseOptionsMap(src map[string]string) []string {
	// 1. Do a first pass getting only the keys.
	dst := make([]string, 0, len(src))
	for k, _ := range src {
		dst = append(dst, k)
	}
	// 2. Sort keys in lexicographic order.
	sort.Strings(dst)
	// 3. Replace each position with key=value.
	for i, k := range dst {
		dst[i] = k + "=" + src[k]
	}
	return dst
}

// CanonicalBuildID generates a canonical hash for the build, useful for caching
// build artifacts and outputs.
//
// TODO this does not take into account the source code of the test plan, nor
// the build configuration per manifest. It should.
func CanonicalBuildID(opts *Input) string {
	hash := sha256.New()
	w := bufio.NewWriter(hash)
	for _, v := range CollapseOptionsMap(opts.Dependencies) {
		w.WriteString(v)
	}
	for _, v := range CollapseOptionsMap(opts.BuildConfigOverride) {
		w.WriteString(v)
	}
	h := hash.Sum(nil)[:hash.Size()]
	hex := strings.ToLower(hex.EncodeToString(h))
	return fmt.Sprintf("testground-%s:%s", opts.TestPlan.Name, hex)
}
