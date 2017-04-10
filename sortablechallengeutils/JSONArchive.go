package sortablechallengeutils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// JSONArchive struct to hold archive file.
type JSONArchive struct {
	ArchiveFileName  string
	ArchiveSourceURL string
	OsExit           func(int)
	OsCreate         func(string) (*os.File, error)
	OsOpen           func(string) (*os.File, error)
	HTTPGet          func(string) (*http.Response, error)
}

// downloadArchive Download the archive file
func (jArchive *JSONArchive) downloadArchive() (archive *os.File, err error) {
	getPackageResponse, err := jArchive.HTTPGet(jArchive.ArchiveSourceURL)
	if err != nil {
		fmt.Println("Error while downloading archive file:", jArchive.ArchiveSourceURL, ", error:", err)
		return nil, err
	}
	packageFile, err := jArchive.OsCreate(jArchive.ArchiveFileName)
	if err != nil {
		fmt.Println("Error creating archive file:", jArchive.ArchiveFileName, ", error:", err)
		return nil, err
	}
	bytesWritten, err := io.Copy(packageFile, getPackageResponse.Body)
	if err != nil {
		fmt.Println("Error while downloading archive file:", jArchive.ArchiveSourceURL, ", error:", err)
		return nil, err
	}
	if err = packageFile.Close(); err != nil {
		fmt.Println("Error closing new archive file:", jArchive.ArchiveFileName, ", error:", err)
		return nil, err
	}
	fmt.Println(bytesWritten, "bytes downloaded to", jArchive.ArchiveFileName)
	archive, err = jArchive.OsOpen(jArchive.ArchiveFileName)
	return archive, err
}

// extractArchiveFile extracts the given file from the archive
func (jArchive *JSONArchive) extractArchiveFile(fileName string) (archivedfile *os.File, err error) {
	archive, err := jArchive.OsOpen(jArchive.ArchiveFileName)
	if os.IsNotExist(err) {
		archive, err = jArchive.downloadArchive()
	}
	if err != nil {
		fmt.Println("Error opening archive file:", jArchive.ArchiveFileName, ", error:", err)
		return nil, err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		fmt.Println("Error creating gzip reader for archive:", jArchive.ArchiveFileName, ", error:", err)
		return nil, err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			fmt.Println("Error could not find file", fileName, "in archive", jArchive.ArchiveFileName)
			break
		}
		if err != nil {
			fmt.Println("Error reading from archive file:", jArchive.ArchiveFileName, ", error:", err)
			return nil, err
		}
		if tarHeader.Typeflag == tar.TypeReg && tarHeader.Name == fileName {
			newFile, err := jArchive.OsCreate(tarHeader.Name)
			if err != nil {
				fmt.Println("Error creating", tarHeader.Name, err)
				return nil, err
			}
			fileSize, err := io.Copy(newFile, tarReader)
			if err != nil {
				fmt.Println("Error writing to", tarHeader.Name, err)
				return nil, err
			}
			if err = newFile.Close(); err != nil {
				fmt.Println("Error closing new file:", tarHeader.Name, err)
				return nil, err
			}
			fmt.Println(fileSize, "bytes saved to file", tarHeader.Name)
			break
		}
	}
	return jArchive.OsOpen(fileName)
}

// ImportJSONFromArchiveFile decodes JSON data from file specified by jsonDecoder
func (jArchive *JSONArchive) ImportJSONFromArchiveFile(fileName string, decodeFunction func(interface {
	Decode(interface{}) error
}) error) (err error) {
	archivedFile, err := jArchive.OsOpen(fileName)
	if os.IsNotExist(err) {
		archivedFile, err = jArchive.extractArchiveFile(fileName)
	}
	if err != nil {
		fmt.Println("Error opening archived file:", fileName, ", error:", err)
		return err
	}
	defer archivedFile.Close()
	decoder := json.NewDecoder(archivedFile)
	for {
		err = decodeFunction(decoder)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
	}
	return
}
