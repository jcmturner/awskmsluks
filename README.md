# AWS KMS LUKS

Use AWS KMS to encrypt Linux block devices with LUKS.

Encryption keys are not stored anywhere in an unencrypted form.

## Setup
In the same AWS region create:
* S3 bucket - this is the backup archive of encrypted keys.
* KMS CMK - this is the CMK that data keys will be created off and encrypted by.

### AWS IAM Permissions
Create an IAM user with the policy found in iam/policy.json

Create an AWS key pair for this user.

### AWS Credentials
On the host that will have the encrypted volume configure the AWS credentials under the root user.
This is described at (https://docs.aws.amazon.com/cli/latest/userguide/cli-config-files.html).
The default region is also needs to be set to the region in which the backup archive bucket and KMS CMK has been created.


### awskmsluks Configuration
Edit the file ``/etc/awskmsluks/config.json`` and set the following values:

* CMKARN: This is the full ARN of the CMK you want to create data keys from for encrypting devices on this host.
* Production: This is a boolean to indicate if this host is considered a production host.
* KeyArchiveBucket: This is the bucket name (not the full ARN) of the bucket to use for keeping an off host backup archive of encrypted data keys.

## Creating an Encrypted Volume
Ecrypt the block device with LUKS using an AWS KMS data key:

```/etc/awskmsluks/bin/awskmsluks -encrypt=/dev/sdb```

Open the device:

```/etc/awskmsluks/bin/awskmsluks -open```

Format the device with the filesystem of your choice.
The open device will be in ``/dev/mapper`` with the name of the device appended with ``_crypt``
For example: 

```mkfs.ext4 /dev/mapper/sdb_crypt```

Create a systemd mount. Set the values in the ``[Mount]`` section of the example below as required.
It is important to have the ``After=awskmsluks.service`` configuration

```
[Unit]
After=awskmsluks.service

[Mount]
What=/dev/mapper/sdb_crypt
Where=/mnt/data
Type=ext4
Options=defaults

[Install]
WantedBy=multi-user.target
```