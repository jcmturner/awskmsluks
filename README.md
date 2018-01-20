# AWS KMS LUKS

WORK IN PROGRESS

## Setup

### Configure AWS KMS LUKS

### Format Encrypted Device
```bash
awskmsluks -format | cryptsetup luksFormat <device> - 
cryptsetup luksUUID <device> --uuid xxxxx
```

### Openning Device
```bash
awskmsluks -uuid=$(blkid -o value -s UUID <device>) | cryptsetup --key-file=- luksOpen <device> <name> 
```