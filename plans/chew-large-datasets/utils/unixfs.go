package utils

import (
	// "io"
	"os"

	files "github.com/ipfs/go-ipfs-files"
)

// ConvertFileToUnixfsFile converts os.File into a Unixfs friendly instance
/* WIP
func ConvertFileToUnixfsFile(path string, file os.File) (files.File, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	var fileReader io.ReadCloser
	fileReader = file.Fd()

	f, err := files.NewReaderPathFile(path, fileReader, stat)
	if err != nil {
		return nil, err
	}

	return f, nil
}
*/

// GetPathToUnixfsFile returns a Unixfs friendly instance of a file located on a path
func GetPathToUnixfsFile(path string) (files.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	unixfsFile, err := files.NewReaderPathFile(path, file, stat)
	if err != nil {
		return nil, err
	}

	return unixfsFile, nil
}

// GetPathToUnixfsDirectory returns a Unixfs friendly instance of a directory located on a path
func GetPathToUnixfsDirectory(path string) (files.Node, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	unixfsDirectory, err := files.NewSerialFile(path, false, stat)
	if err != nil {
		return nil, err
	}

	return unixfsDirectory, nil
}
