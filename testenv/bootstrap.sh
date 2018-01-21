#!/bin/bash

rm /etc/localtime
ln -s /usr/share/zoneinfo/Europe/London /etc/localtime

yum upgrade -y
yum install -y \
  cryptsetup \
  git \
  ntp \
  epel-release \
  net-tools

yum install -y python2-pip

systemctl stop firewalld
systemctl disable firewalld

pip install awscli

mkdir -p /etc/awskmsluks/bin
cp /vagrant/config.json /etc/awskmsluks/
cp /vagrant/awskmsluks /etc/awskmsluks/bin/

mkdir /mnt/data

cp /vagrant/awskmsluks.service /etc/systemd/system/
cp /vagrant/mnt-data.mount /etc/systemd/system/

systemctl enable awskmsluks.service mnt-data.mount

cat <<EOF >> /etc/sysctl.conf
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
net.ipv6.conf.lo.disable_ipv6 = 1 
EOF

#Turning off selinux 
sed -i "s/SELINUX=.*/SELINUX=permissive/g" /etc/sysconfig/selinux
sed -i "s/SELINUX=.*/SELINUX=permissive/g" /etc/selinux/config

echo 'Set up AWS credentials as root by running "aws configure"' 1>&2

reboot
