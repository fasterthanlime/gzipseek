package main

import (
	"io"
	"log"
	"os"
	"strings"

	"github.com/itchio/arkive/zip"
	"github.com/itchio/kompress/flate"

	humanize "github.com/dustin/go-humanize"
)

func main() {
	fPath := os.Args[1]

	eName := ""
	if len(os.Args) > 2 {
		eName = os.Args[2]
	}

	zf, err := os.Open(fPath)
	if err != nil {
		panic(err)
	}

	fStats, err := zf.Stat()
	if err != nil {
		panic(err)
	}

	zr, err := zip.NewReader(zf, fStats.Size())
	if err != nil {
		panic(err)
	}

	totalCheckpoints := 0

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if !strings.Contains(f.Name, eName) {
			continue
		}

		if f.Method != zip.Deflate {
			log.Printf("Not compressed with deflate: %s", f.Name)
			continue
		}

		offset, err := f.DataOffset()
		if err != nil {
			panic(err)
		}

		sr := io.NewSectionReader(zf, offset, int64(f.CompressedSize64))

		fr := flate.NewSaverReader(sr)

		var mb int64 = 1024 * 1024
		var readBytes int64
		var totalReadBytes int64

		var checkpoints []*flate.Checkpoint
		var c *flate.Checkpoint

		buf := make([]byte, 1<<15)

		for {
			if readBytes > mb {
				fr.WantSave()
			}

			n, err := fr.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				} else if err == flate.ReadyToSaveErr {
					c, err = fr.Save()
					if err != nil {
						panic(err)
					}
					fr.Close()

					log.Printf("Saved checkpoint %s", c)

					checkpoints = append(checkpoints, c)

					sr = io.NewSectionReader(zf, offset, int64(f.CompressedSize64))

					fr, err = c.Resume(sr)
					if err != nil {
						panic(err)
					}

					totalReadBytes += readBytes
					readBytes = 0
				} else {
					panic(err)
				}
			}
			readBytes += int64(n)
		}

		totalReadBytes += readBytes

		log.Printf("%s: Read %s uncompressed data", f.Name, humanize.IBytes(uint64(totalReadBytes)))
		log.Printf("%s: Created %d checkpoints", f.Name, len(checkpoints))

		totalCheckpoints += len(checkpoints)

		err = fr.Close()
		if err != nil {
			panic(err)
		}
	}

	log.Printf("Across all files, created %d checkpoints", totalCheckpoints)
}
