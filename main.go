package main

import (
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

/*
Source i have use for write this code:
https://github.com/turbospok/Flipper-NTAG215-password-converter/blob/main/ntag215converter.py
https://www.nxp.com/docs/en/data-sheet/NTAG213_215_216.pdf
https://github.com/Lucaslhm/AmiiboFlipperConverter/blob/main/src/amiiboconvert.py
https://www.reddit.com/r/flipperzero/comments/tfy0ta/load_nfc_bin_files_directly_to_flipper_zero/
https://github.com/AmiiboDB/Amiibo
https://github.com/Lanjelin/AmiiboConverter/blob/main/AmiiboConverter.py
https://www.reddit.com/r/flipperzero/comments/ydlytv/comment/ksxtn7j/?utm_source=share&utm_medium=web3x&utm_name=web3xcss&utm_term=1&utm_content=share_button

*/

const (
	// Amiibos uses NTAG215, NTAG215 Tag has 135 pages (0 to 134), each pages contains 4 bytes
	// for more information check de ntag215 documentation
	Ntag215PageQuantity = 135
)

func loadBinFile(filename string) ([]byte, error) {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return []byte{}, errors.New("error when loading file, check filename")
	}
	return fileContent, nil
}

func saveNfcFile(filename string, content []byte) error {
	if err := os.WriteFile(filename, content, 0644); err != nil {
		return errors.New("error when write nfc file")
	}
	return nil
}

func extractUid(fileContent []byte) string {
	// Amiibo Uid is the addition of first 3 bytes  and 4 to 7 bytes of bin file
	// More simple: This is the first 8 bytes and we remove the bytes at position 3
	uidBytes := slices.Concat(fileContent[:3], fileContent[4:8])
	strUid := []string{}
	for _, b := range uidBytes {
		strUid = append(strUid, strings.ToUpper(hex.EncodeToString([]byte{b})))
	}
	return strings.Join(strUid, " ")
}

func calculatePassword(rawUid string) string {
	// calculate the password of the tag from the bin file content
	password := []string{}
	password = append(password, hex.EncodeToString([]byte{rawUid[1] ^ rawUid[3] ^ 0xAA}))
	password = append(password, hex.EncodeToString([]byte{rawUid[2] ^ rawUid[4] ^ 0x55}))
	password = append(password, hex.EncodeToString([]byte{rawUid[3] ^ rawUid[5] ^ 0xAA}))
	password = append(password, hex.EncodeToString([]byte{rawUid[4] ^ rawUid[6] ^ 0x55}))
	return strings.Join(password, " ")
}

func convertBinDataToNfcPages(fileContent []byte) string {
	pagesContent := []string{}
	pageCount := 0

	if len(fileContent)%4 != 0 {
		// case where the file as missing bytes, pad the content with 00 bytes...
		for len(fileContent)%4 != 0 {
			fileContent = append(fileContent, byte('\x00'))
		}
	}

	for i := 0; i < len(fileContent); i += 4 {
		page := "Page " + strconv.FormatInt(int64(pageCount), 10) + ":"

		for j := 0; j < 4; j++ {
			page += " " + strings.ToUpper(hex.EncodeToString(fileContent[i+j:i+j+1]))
		}
		pagesContent = append(pagesContent, page)
		pageCount++
		if pageCount >= Ntag215PageQuantity {
			// some amiibo bins are in 572 bytes, if the content is too big ignore the bytes
			break
		}
	}
	if pageCount < Ntag215PageQuantity {
		// if the file is too small, pad the page with 00 bytes
		for pageCount < Ntag215PageQuantity {
			page := "Page " + strconv.FormatInt(int64(pageCount), 10) + ": 00 00 00 00"
			pagesContent = append(pagesContent, page)
			pageCount++
		}
	}

	uid, _ := hex.DecodeString(strings.ReplaceAll(extractUid(fileContent), " ", ""))
	pagesContent[133] = "Page 133: " + strings.ToUpper(calculatePassword(string(uid)))
	pagesContent[134] = "Page 134: 80 80 00 00"

	return strings.Join(pagesContent, "\n")
}

func createNfcFileContent(uid string, pages string) string {
	content := fmt.Sprintf(`Filetype: Flipper NFC device
Version: 2
# Nfc device type can be UID, Mifare Ultralight, Bank card
Device type: NTAG215
# UID, ATQA and SAK are common for all formats
UID: %s
ATQA: 44 00
SAK: 00
# Mifare Ultralight specific data
Signature: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
Mifare version: 00 04 04 02 01 00 11 03
Counter 0: 0
Tearing 0: 00
Counter 1: 0
Tearing 1: 00
Counter 2: 0
Tearing 2: 00
Pages total: %d
%s`, uid, Ntag215PageQuantity, pages)
	return content
}

func getAllBinFiles(path string) []string {
	files := []string{}
	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, ".bin") && !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}
	return files
}

func main() {
	inputDir := flag.String("input", "Amiibo Bins", "Input path")
	outputDir := flag.String("output", "output", "Path for converted Files")

	flag.Parse()
	filesToConvert := getAllBinFiles(*inputDir)

	for _, file := range filesToConvert {

		fileContent, err := loadBinFile(file)
		if err != nil {
			log.Panicln(err)
		}

		nfcContent := createNfcFileContent(string(extractUid(fileContent)), convertBinDataToNfcPages(fileContent))

		fmt.Println("Proccess file: ", file)
		outputPath := strings.ReplaceAll(file, *inputDir, *outputDir)
		outputPath = strings.ReplaceAll(outputPath, ".bin", ".nfc")
		os.MkdirAll(filepath.Dir(outputPath), 0644)

		if err := saveNfcFile(outputPath, []byte(nfcContent)); err != nil {
			log.Println("Error when save NfcFile ", err)
		}
	}
}
