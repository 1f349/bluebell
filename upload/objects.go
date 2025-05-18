package upload

import (
	"crypto/sha256"
	"fmt"
	"github.com/1f349/bluebell/logger"
	"github.com/google/uuid"
	"github.com/spf13/afero"
	"io"
	"os"
	"path/filepath"
)

type Hash [sha256.Size]byte

func (h Hash) String() string {
	return fmt.Sprintf("%02x", h[:])
}

func (h Hash) ObjectPath() string {
	return fmt.Sprintf("%02x/%02x/%02x", h[0], h[1], h[2:])
}

type ObjectStore struct {
	dir afero.Fs
}

func NewObjectStore(dir afero.Fs) *ObjectStore {
	return &ObjectStore{dir: dir}
}

func (o *ObjectStore) writeTempFile(r io.Reader) (string, error) {
	// make temp dir
	err := o.dir.MkdirAll("temp", 0700)
	if err != nil {
		return "", err
	}

	// make temp upload file
	tempPath := filepath.Join("temp", "upload-"+uuid.NewString())
	file, err := o.dir.OpenFile(tempPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = io.Copy(file, r)
	if err != nil {
		return "", err
	}
	return tempPath, nil
}

// AddReader creates a file in the object store from an io.Reader
func (o *ObjectStore) AddReader(r io.Reader) (Hash, error) {
	h := sha256.New()

	fmt.Println("AddReader")

	// write to hash as well
	r = io.TeeReader(r, h)

	tempFileName, err := o.writeTempFile(r)
	if err != nil {
		logger.Logger.Error("Failed to create temp file", "err", err)
		return Hash{}, err
	}
	fmt.Println(o.dir)

	objName := Hash(h.Sum(nil))
	objPath := objName.ObjectPath()
	fmt.Println(objPath)
	fmt.Println(tempFileName)

	err = o.dir.MkdirAll(filepath.Dir(objPath), 0700)
	if err != nil {
		return Hash{}, err
	}

	err = o.dir.Rename(tempFileName, objName.ObjectPath())
	if err != nil {
		return Hash{}, err
	}

	return objName, nil
}

func (o *ObjectStore) Open(hash Hash) (io.Reader, error) {
	return o.dir.Open(hash.ObjectPath())
}

func (o *ObjectStore) Remove(hash Hash) error {
	return o.dir.Remove(hash.ObjectPath())
}
