package config

import "path/filepath"

type Directories struct {
	home string
}

func (d Directories) Home() string {
	return d.home
}

func (d Directories) Plans() string {
	return filepath.Join(d.home, "plans")
}

func (d Directories) SDKs() string {
	return filepath.Join(d.home, "sdks")
}

func (d Directories) Work() string {
	return filepath.Join(d.home, "data", "work")
}

func (d Directories) Outputs() string {
	return filepath.Join(d.home, "data", "outputs")
}
