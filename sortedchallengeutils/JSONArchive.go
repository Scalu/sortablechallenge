package sortedchallengeutils

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type jsonDecoder interface {
	fileName() string
	decode(*json.Decoder) error
}

// JSONArchive struct to hold archive file.
type JSONArchive struct {
	ArchiveFileName  string
	ArchiveSourceURL string
}

func (jArchive *JSONArchive) downloadArchive() (archive *os.File, err error) {
	getPackageResponse, err := http.Get(jArchive.ArchiveSourceURL)
	if err != nil {
		fmt.Println("Error while downloading archive file: ", jArchive.ArchiveSourceURL, err)
		return nil, err
	}
	packageFile, err := os.Create(jArchive.ArchiveFileName)
	if err != nil {
		fmt.Println("Error creating archive file: ", jArchive.ArchiveFileName, err)
		return nil, err
	}
	bytesWritten, err := io.Copy(packageFile, getPackageResponse.Body)
	if err != nil {
		fmt.Println("Error while downloading archive file: ", jArchive.ArchiveSourceURL, err)
		return nil, err
	}
	if err = packageFile.Close(); err != nil {
		fmt.Println("Error closing new archive file: ", jArchive.ArchiveFileName, err)
		return nil, err
	}
	fmt.Println(bytesWritten, " bytes downloaded to ", jArchive.ArchiveFileName)
	archive, err = os.Open(jArchive.ArchiveFileName)
	return archive, err
}

func (jArchive *JSONArchive) extractArchiveFile(fileName string) (archivedfile *os.File, err error) {
	archive, err := os.Open(jArchive.ArchiveFileName)
	if os.IsNotExist(err) {
		archive, err = jArchive.downloadArchive()
	}
	if err != nil {
		fmt.Println("Error opening archive file: ", jArchive.ArchiveFileName, err)
		return nil, err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		fmt.Println("Error creating gzip reader for archive: ", jArchive.ArchiveFileName, err)
		return nil, err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		tarHeader, err := tarReader.Next()
		if err == io.EOF {
			fmt.Println("Error could not find file ", fileName, " in archive ", jArchive.ArchiveFileName)
			break
		}
		if err != nil {
			fmt.Println("Error reading from archive file: ", jArchive.ArchiveFileName, err)
			return nil, err
		}
		if tarHeader.Typeflag == tar.TypeReg && tarHeader.Name == fileName {
			newFile, err := os.Create(tarHeader.Name)
			if err != nil {
				fmt.Println("Error creating ", tarHeader.Name, err)
				return nil, err
			}
			fileSize, err := io.Copy(newFile, tarReader)
			if err != nil {
				fmt.Println("Error writing to ", tarHeader.Name, err)
				return nil, err
			}
			if err = newFile.Close(); err != nil {
				fmt.Println("Error closing new file: ", tarHeader.Name, err)
				return nil, err
			}
			fmt.Println(fileSize, " bytes saved to file ", tarHeader.Name)
			break
		}
	}
	return os.Open(fileName)
}

// ImportJSONFromArchiveFile decodes JSON data from file specified by jsonDecoder
func (jArchive JSONArchive) ImportJSONFromArchiveFile(jDecoder jsonDecoder) (err error) {
	archivedFile, err := os.Open(jDecoder.fileName())
	if os.IsNotExist(err) {
		archivedFile, err = jArchive.extractArchiveFile(jDecoder.fileName())
	}
	if err != nil {
		fmt.Println("Error opening archived file: ", jDecoder.fileName(), err)
		return err
	}
	defer archivedFile.Close()
	decoder := json.NewDecoder(archivedFile)
	for {
		err = jDecoder.decode(decoder)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
	}
	return
}
