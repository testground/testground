package tpipeline

import (
	"io/ioutil"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

const IpfsRepoUrl = "https://github.com/ipfs/go-ipfs"

func CreateTempDir() (string, error) {
	return ioutil.TempDir("", "test-pipeline")
}

func CheckoutCommit(hash string, dir string) error {
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{URL: IpfsRepoUrl})
	if err != nil {
		return errors.Wrap(err, "unable to clone repo")
	}

	tree, err := repo.Worktree()
	if err != nil {
		return errors.Wrap(err, "unable to get work tree")
	}

	err = tree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(hash),
	})
	return errors.Wrapf(err, "unable to checkout specified hash %s", hash)
}
