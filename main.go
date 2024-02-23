package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/parnurzeal/gorequest"
)

type SigninData struct {
	AccessToken string `json:"accessToken"`
}

type VerifyData struct {
	NfsUrl string `json:"nfsUrl"`
}

const (
	baseURL = "http://localhost:8080/api/auth"
)

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func getAccessToken() (string, error) {
	signinURL := baseURL + "/signin"
	signinData := map[string]interface{}{
		"username": "test",
		"password": "test",
	}

	resp, body, errs := gorequest.New().
		Post(signinURL).
		Send(signinData).
		End()

	if errs != nil {
		log.Fatalf("Error sending Signin request: %v", errs)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("Signin request failed with status code: %d", resp.StatusCode)
	}

	var signinResponse map[string]interface{}
	if err := json.Unmarshal([]byte(body), &signinResponse); err != nil {
		log.Fatalf("Error parsing Signin response: %v", err)
	}

	signinDataResponse, ok := signinResponse["data"].(map[string]interface{})
	if !ok {
		log.Fatal("Invalid Signin response format")
	}

	return signinDataResponse["accessToken"].(string), nil
}

func getNfsUrl(accessToken string) (string, error) {
	verifyURL := baseURL + "/verify"
	verifyHeader := map[string]string{
		"Authorization": "Bearer " + accessToken,
	}

	resp, body, errs := gorequest.New().
		Get(verifyURL).
		Set("Authorization", verifyHeader["Authorization"]).
		End()

	if errs != nil {
		log.Fatalf("Error sending Verify request: %v", errs)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("Verify request failed with status code: %d", resp.StatusCode)
	}

	var verifyResponse map[string]interface{}
	if err := json.Unmarshal([]byte(body), &verifyResponse); err != nil {
		log.Fatalf("Error parsing Verify response: %v", err)
	}

	verifyDataResponse, ok := verifyResponse["data"].(map[string]interface{})
	if !ok {
		log.Fatal("Invalid Verify response format")
	}

	return verifyDataResponse["nfsUrl"].(string), nil
}

func main() {
	loadEnv()

	accessToken, err := getAccessToken()
	if err != nil {
		log.Fatal(err)
	}

	nfsUrl, err := getNfsUrl(accessToken)

	fmt.Printf("NFS URL: %s\n", nfsUrl)
}
