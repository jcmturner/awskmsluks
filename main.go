package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

const (
	dirRoot    = "/etc/awskmsluks/"
	configPath = "config.json"
	keyStore   = "keys/"
	cryptsetup = "/sbin/cryptsetup"
)

var buildhash = "Not set"
var buildtime = "Not set"
var version = "Not set"

func version() (string, string, time.Time) {
	bt, _ := time.Parse(time.RFC3339, buildtime)
	return version, buildhash, bt
}

func main() {
	encrypt := flag.String("encrypt", "", "Generate passphrase for LUKS and encrypt the device")
	open := flag.Bool("open", false, "Open all encrypted devices")
	//clse := flag.Bool("close", false, "Close all encrypted devices")
	uuid := flag.String("uuid", "", "Return the passphrase to open the LUKS device. Value must be the UUID of the device")
	version := flag.Bool("version", false, "Print version information")
	flag.Parse()

	// Print version information and exit.
	if *version {
		v, bh, bt := version()
		fmt.Fprintf(os.Stderr, "AWS KMS LUKS Version Information:\nVersion:\t%s\nBuild hash:\t%s\nBuild time:\t%v\n", v, bh, bt)
		os.Exit(0)
	}

	if *encrypt == "" && !*open && *uuid == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *encrypt != "" {
		err := formatDev(*encrypt)
		if err != nil {
			panic(err.Error())
		}
	}
	if *open {
		openAll()
	}
	if *uuid != "" {
		err := passPhrase(*uuid)
		if err != nil {
			panic(err.Error())
		}
	}
}

func formatDev(dev string) error {
	if !strings.HasPrefix(dev, "/dev/") {
		return fmt.Errorf("device %s is not vaid", dev)
	}
	// Check device exists
	if _, err := os.Stat(dev); os.IsNotExist(err) {
		return fmt.Errorf("device %s does not exist", dev)
	}
	// Get host's FQDN
	fqdn, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("could not get host's FQDN: " + err.Error())
	}
	// Load configuration
	c, err := loadConfig(dirRoot + configPath)
	if err != nil {
		return err
	}
	// Generate a new data key
	key, err := newDataKey(c.CMKARN, fqdn, c.Production)
	if err != nil {
		return fmt.Errorf("could not generate the data key: " + err.Error())
	}
	// Archive the key to S3
	err = key.archive(c.KeyArchiveBucket)
	if err != nil {
		return fmt.Errorf("could not archive the data key: " + err.Error())
	}
	// Store the key locally
	err = key.store(dirRoot + keyStore)
	if err != nil {
		return err
	}

	// Perform the cryptosetup format
	cmd := exec.Command(cryptsetup, "--key-file=-", "luksFormat", dev, "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open stdin to cryptsetup command: %v", err)
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start cryptsetup command: %v", err)
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, key.DataKey.Plain)
	}()
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("cyrptsetup did not run successfully to format device %s: %v", dev, err)
	}

	// Set the UUID on the device
	cmd = exec.Command(cryptsetup, "luksUUID", dev, "--uuid", key.EncryptionContext.UUID)
	stdin, err = cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open stdin to cryptsetup command: %v", err)
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start cryptsetup command: %v", err)
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, "YES\n")
	}()
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("cyrptsetup did not run successfully to set the UUID of device %s: %v. Suggest running manually:\ncryptsetup luksUUID <device> --uuid %s", dev, err, key.EncryptionContext.UUID)
	}
	fmt.Printf("Successfully encrypted device %s\n", dev)
	fmt.Println("Device now needs to be formated with a filesystem as usual")

	return nil
}

func passPhrase(uuid string) error {
	// Get host's FQDN
	fqdn, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("could not get host's FQDN: " + err.Error())
	}

	kpath := dirRoot + keyStore + fqdn + "/" + uuid + ".json"
	var k key
	err = k.Load(kpath)
	if err != nil {
		return fmt.Errorf("error loading key: %v", err)
	}
	err = k.decrypt()
	if err != nil {
		return fmt.Errorf("could not decrypt key: %v", err)
	}
	fmt.Print(k.DataKey.Plain)
	return nil
}

func openDev(k key) error {
	dev, err := devFromUUID(k.EncryptionContext.UUID)
	if err != nil {
		return fmt.Errorf("could not get device name for UUID %s: %v", k.EncryptionContext.UUID, err)
	}
	err = k.decrypt()
	if err != nil {
		return fmt.Errorf("could not decrypt key for UUID %s: %v", k.EncryptionContext.UUID, err)
	}
	name := path.Base(dev) + "_crypt"
	cmd := exec.Command(cryptsetup, "--key-file=-", "luksOpen", dev, name)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("could not open stdin to cryptsetup command: %v", err)
	}

	fmt.Printf("openning UUID %s, device %s to %s\n", k.EncryptionContext.UUID, dev, name)
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start cryptsetup command: %v", err)
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, k.DataKey.Plain)
	}()
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("cyrptsetup did not run successfully for UUID %s device %s: %v", k.EncryptionContext.UUID, dev, err)
	}
	statusCmd := exec.Command(cryptsetup, "status", name)
	stdout, _ := statusCmd.StdoutPipe()
	stderr, _ := statusCmd.StderrPipe()
	go func() {
		defer stdout.Close()
		io.Copy(os.Stdout, stdout)
	}()
	go func() {
		defer stderr.Close()
		io.Copy(os.Stderr, stderr)
	}()
	statusCmd.Run()
	return nil
}

func openAll() {
	ks, err := keys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting device keys: %v\n", err)
	}
	for _, k := range ks {
		err = openDev(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error openning device: %v\n", err)
		}
	}
}
