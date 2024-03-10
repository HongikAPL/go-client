package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	mrand "math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/parnurzeal/gorequest"
	"github.com/pquerna/otp/totp"
)

const (
	baseURL    = "http://15.164.217.15:8080/api/auth"
	folderPath = "./nfs_shared_data"
)

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func generateSeed() (int64, error) {
	signinURL := baseURL + "/signin"
	signinRequestBody := map[string]interface{}{
		"username": "test",
		"password": "test",
	}

	resp, body, errs := gorequest.New().
		Post(signinURL).
		Send(signinRequestBody).
		End()

	if errs != nil {
		log.Fatalf("Error sending Signin request: %v", errs)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("Signin request failed with status code: %d", resp.StatusCode)
	}

	var signinResponse struct {
		Data struct {
			Seed int64  `json:"seed"`
			Key  []byte `json:"Key"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &signinResponse); err != nil {
		log.Fatalf("Error parsing Signin response: %v", err)
	}

	seed := signinResponse.Data.Seed
	key := signinResponse.Data.Key

	fmt.Println("seed :", seed)
	fmt.Println("key :", key)

	return int64(seed), nil
}

func generateOTP(seed int64, secretKeyBytes []byte) (string, error) {
	// otpURL, err := totp.Generate(totp.GenerateOpts{
	// 	Issuer:      "App Name",
	// 	AccountName: "test@example.com",
	// 	Secret:      secretKeyBytes,
	// })
	// if err != nil {
	// 	fmt.Println("Error generating TOTP URL:", err)
	// 	return "", err
	// }

	// fmt.Println("TOTP URL:\n", otpURL.URL())
	secretKey := base32.StdEncoding.EncodeToString(secretKeyBytes)
	secretKey = secretKey[:32]

	fixedTime := time.Unix((time.Now().Unix()/30)*30, 0)

	totp, err := totp.GenerateCode(secretKey, fixedTime)
	if err != nil {
		fmt.Println("Error generating TOTP code:", err)
		return "", err
	}

	return totp, nil
}

func decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := ciphertext[:aes.BlockSize]
	data := ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)

	return data, nil
}

func decryptFilesInFolder(key []byte) error {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath := filepath.Join(folderPath, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}

		decryptedData, err := decrypt(data, key)
		if err != nil {
			return err
		}

		tempFilePath := filePath + ".tmp"
		tempFile, err := os.Create(tempFilePath)
		if err != nil {
			return err
		}
		defer tempFile.Close()

		_, err = tempFile.Write(decryptedData)
		if err != nil {
			return err
		}

		fmt.Println("success write decryptedData")

		err = tempFile.Chmod(0644)
		if err != nil {
			return err
		}

		fmt.Println("success change mode")

		err = os.Remove(filePath)
		if err != nil {
			return err
		}
		err = os.Rename(tempFilePath, filePath)
		if err != nil {
			return err
		}
	}

	return nil
}

func getNfsUrl(otp string) (string, error) {
	verifyURL := baseURL + "/verify"
	verifyRequestBody := map[string]interface{}{
		"otp": otp,
	}

	resp, body, errs := gorequest.New().
		Post(verifyURL).
		Send(verifyRequestBody).
		End()

	if errs != nil {
		log.Fatalf("Error sending Verify request: %v", errs)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("Verify request failed with status code: %d", resp.StatusCode)
	}

	var verifyResponseDto struct {
		Data struct {
			NfsUrl string `json:"nfsUrl"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(body), &verifyResponseDto); err != nil {
		log.Fatalf("Error parsing Signin response: %v", err)
	}

	nfsUrl := verifyResponseDto.Data.NfsUrl

	return nfsUrl, nil
}

func mountNfs(nfsUrl string, otp string) error {
	nfsPath := nfsUrl + "/" + otp
	commands := []string{
		"apt-get update",
		"apt-get install -y nfs-common",
		"mkdir -p nfs_shared_data",
		fmt.Sprintf("mount %s %s", nfsPath, folderPath),
	}
	for _, cmd := range commands {
		err := exec.Command("/bin/sh", "-c", cmd).Run()
		if err != nil {
			return fmt.Errorf("failed to execute command %s: %s", cmd, err)
		}
	}
	return nil
}

func readDatasInFolder() error {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath := filepath.Join(folderPath, file.Name())
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Println("Error reading file", filePath, err)
			continue
		}

		fmt.Println("File :", filePath)
		fmt.Println(string(data))
		fmt.Println("----------------")
	}

	return nil
}

func generateRandomSecretKey(seed int64) ([]byte, error) {
	randSource := mrand.NewSource(seed)
	randInstance := mrand.New(randSource)
	randomSecretKey := make([]byte, 32)

	for i := 0; i < len(randomSecretKey); i++ {
		randomSecretKey[i] = byte(randInstance.Intn(256))
	}

	return randomSecretKey, nil
}

func deleteLink(otp string) error {
	deleteURL := fmt.Sprintf("%s/link/%s", baseURL, otp)

	resp, body, errs := gorequest.New().
		Delete(deleteURL).
		End()

	if errs != nil {
		return fmt.Errorf("Error sending DELETE request: %v", errs)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("DELETE request failed with status code: %d", resp.StatusCode)
	}

	var deleteResponseDto struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal([]byte(body), &deleteResponseDto); err != nil {
		return err
	}

	fmt.Println(deleteResponseDto.Message)

	return nil
}

func main() {
	loadEnv()

	seed, err := generateSeed()
	if err != nil {
		log.Fatal(err)
	}

	key, err := generateRandomSecretKey(seed)
	if err != nil {
		log.Fatal(err)
	}

	totp, err := generateOTP(seed, key)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("TOTP :", totp)

	nfsUrl, err := getNfsUrl(totp)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("NFS URL :", nfsUrl)

	if err = mountNfs(nfsUrl, totp); err != nil {
		log.Fatal(err)
	}

	fmt.Println("NFS mounted successfully!")

	if err = decryptFilesInFolder(key); err != nil {
		log.Fatal(err)
	}

	if err = readDatasInFolder(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Read Data successfully!")

	if err := deleteLink(totp); err != nil {
		log.Fatal(err)
	}
}
