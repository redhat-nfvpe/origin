#!/bin/bash

# Iterate over /sys/class/net,
# Provision VFs on interfaces with following properties:
# 1) SR-IOV capable interface
# 2) Link state up
# 3) No default route configured on interface
# 4) Of certain NIC types

set -x

progname=$0
NUMVF=2
VENDORID=
DEVICEID=
INT=

function usage () {
   cat <<EOF
Usage: $progname [-c num_vfs]
EOF
   exit 0
}

while getopts c:v:i:d:h FLAG; do
   case $FLAG in

   c)  echo "Creating $OPTARG VF(s)"
       NUMVF=$OPTARG
       ;;
   v)  echo "Vendor ID specified $OPTARG"
       VENDORID=$OPTARG
       ;;
   d)  echo "Device ID specified $OPTARG"
       DEVICEID=$OPTARG
       ;;
   i)  echo "Creating VF on $OPTARG interface"
       INT=$OPTARG
       ;;
   h) usage ;;
   *) usage ;;
   esac
done

if [ -n "$INT" ]; then
	if [ -e /sys/class/net/$INT/device/sriov_numvfs ]; then
		if [ $(echo $NUMVF > /sys/class/net/$INT/device/sriov_numvfs) ]; then
			exit
		fi
	fi
	echo "failed to configure $NUMVF vfs on $INT interface, exiting"
	exit 1
fi

for i in `ls /sys/class/net`
do
	# Skip interface without SR-IOV capability
	if [ ! -e /sys/class/net/$i/device/sriov_numvfs ]; then
		continue
	fi

	# Skip interface with operstate being 'down'
	if [ $(cat /sys/class/net/$i/operstate) == 'down' ]; then
		continue
	fi

	# Skip interface with ip configured
	if [ $(ip route list | grep -q $i) ]; then
		continue
	fi

	# Skip interface not from vendor id
	if [ "$(cat /sys/class/net/$i/device/vendor)" != "$VENDORID" ]; then
		continue
	fi

	# Skip interface not with device id
	if [ "$(cat /sys/class/net/$i/device/device)" != "$DEVICEID" ]; then
		continue
	fi

	if [ $(echo $NUMVF > /sys/class/net/$i/device/sriov_numvfs) ]; then
		echo "failed to configure $NUMVF vfs on $i interface, exiting"
		exit 1
	else
		echo "successfully configured $NUMVF vfs on $i interface"
		exit
	fi
done
