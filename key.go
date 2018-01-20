package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/s3/s3manager"
	"github.com/hashicorp/go-uuid"
	"github.com/jcmturner/awsarn"
	"os"
	"strconv"
	"time"
)

const (
	passPhraseByteSize = 1024
)

type key struct {
	EncryptionContext encryptionContext `json:"EncryptionContext"`
	CMKARN            string            `json:"CMKARN"`
	DataKey           dataKey           `json:"DataKey"`
}

type encryptionContext struct {
	FQDN       string `json:"FQDN"`
	Production bool   `json:"Production"`
	UUID       string `json:"UUID"`
}

type dataKey struct {
	Plain     string    `json:"Plain,omitempty"`
	Encrypted string    `json:"Encrypted"`
	Created   time.Time `json:"Created"`
}

func newEncryptionContext(fqdn, uuid string, production bool) encryptionContext {
	return encryptionContext{
		FQDN:       fqdn,
		Production: production,
		UUID:       uuid,
	}
}

func (e encryptionContext) toMap() map[string]string {
	return map[string]string{
		"fqdn":       e.FQDN,
		"production": strconv.FormatBool(e.Production),
		"uuid":       e.UUID,
	}
}

func newDataKey(cmkARNStr, fqdn string, production bool) (key, error) {
	cmkARN, err := awsarn.Parse(cmkARNStr, nil)
	if err != nil {
		return key{}, fmt.Errorf("invalid CMK ARN: %v", err)
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return key{}, fmt.Errorf("unable to load AWS SDK config: %v", err)
	}
	cfg.Region = cmkARN.Region

	kmsSrv := kms.New(cfg)

	devUUID, err := uuid.GenerateUUID()
	if err != nil {
		return key{}, err
	}
	ec := newEncryptionContext(fqdn, devUUID, production)
	bs := int64(passPhraseByteSize)
	input := kms.GenerateDataKeyInput{
		EncryptionContext: ec.toMap(),
		KeyId:             &cmkARNStr,
		NumberOfBytes:     &bs,
	}
	request := kmsSrv.GenerateDataKeyRequest(&input)
	output, err := request.Send()
	if err != nil {
		return key{}, err
	}

	k := key{
		EncryptionContext: ec,
		CMKARN:            cmkARNStr,
		DataKey: dataKey{
			Plain:     base64.StdEncoding.EncodeToString(output.Plaintext),
			Encrypted: base64.StdEncoding.EncodeToString(output.CiphertextBlob),
			Created:   time.Now().UTC(),
		},
	}
	return k, nil
}

func (k key) archive(bucket string) error {
	//Blank the plaintext form of the key before storing
	k.DataKey.Plain = ""
	// The config the S3 Uploader will use
	cfg, err := external.LoadDefaultAWSConfig()

	// Create an uploader with the config and default options
	uploader := s3manager.NewUploader(cfg)

	// Marshal the key to json
	keyBytes, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal key: %v", err)
	}

	// Upload the file to S3.
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(fmt.Sprintf("%s/%s.json", k.EncryptionContext.FQDN, k.EncryptionContext.UUID)),
		Body:   bytes.NewBuffer(keyBytes),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	return nil
}

func (k key) store(path string) error {
	k.DataKey.Plain = ""
	err := os.MkdirAll(path+k.EncryptionContext.FQDN, 0600)
	if err != nil {
		return fmt.Errorf("could not create local key store directory: " + err.Error())
	}
	kjson, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal key to JSON: " + err.Error())
	}
	kf, err := os.OpenFile(path+k.EncryptionContext.FQDN+"/"+k.EncryptionContext.UUID+".json", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("could not open key file: " + err.Error())
	}
	defer kf.Close()
	_, err = kf.Write(kjson)
	if err != nil {
		return fmt.Errorf("could not write to local key store: %v", err)
	}
	return nil
}

func (k *key) decrypt() error {
	cmkARN, err := awsarn.Parse(k.CMKARN, nil)
	if err != nil {
		return fmt.Errorf("invalid CMK ARN: %v", err)
	}

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return fmt.Errorf("unable to load AWS SDK config: %v", err)
	}
	cfg.Region = cmkARN.Region

	kmsSrv := kms.New(cfg)

	b, err := base64.StdEncoding.DecodeString(k.DataKey.Encrypted)
	if err != nil {
		return fmt.Errorf("cannot base64 decode encrypted key: %v", err)
	}

	input := kms.DecryptInput{
		CiphertextBlob:    b,
		EncryptionContext: k.EncryptionContext.toMap(),
	}
	request := kmsSrv.DecryptRequest(&input)
	output, err := request.Send()
	if err != nil {
		return err
	}
	k.DataKey.Plain = base64.StdEncoding.EncodeToString(output.Plaintext)
	return nil
}
