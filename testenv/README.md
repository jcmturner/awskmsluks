
* Build binary called awskmsluks in the testenv directory
* Configure the config.json file for:
  * CMK ARN
  * Archive bucket name
* Copy the ../systemd files into the testenv directory
* Once the vagrant image is up log in switch to root and run "aws configure".
Set the access key and secret key and the region of the archive bucket as the default region.