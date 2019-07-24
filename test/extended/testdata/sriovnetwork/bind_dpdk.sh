#!/bin/bash

# Iterate over /sys/class/net and Bind VFs to vfio-pci driver

set -x

progname=$0
Bind=
unBind=
failCount=0

function usage () {
   cat <<EOF
Usage: $progname [-b]
EOF
   exit 0
}

function BindVFToDPDK () {
	pci=$1
	ret=0
	chroot /host /bin/bash -c "echo $pci > /sys/bus/pci/drivers/iavf/unbind"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	chroot /host /bin/bash -c "echo vfio-pci > /sys/bus/pci/devices/$pci/driver_override"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	chroot /host /bin/bash -c "echo $pci > /sys/bus/pci/drivers/vfio-pci/bind"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	chroot /host /bin/bash -c "echo  > /sys/bus/pci/devices/$pci/driver_override"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	echo $ret
}

function unBindVF () {
	pci=$1
	ret=0
	chroot /host /bin/bash -c "echo $pci > /sys/bus/pci/drivers/vfio-pci/unbind"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	chroot /host /bin/bash -c "echo iavf > /sys/bus/pci/devices/$pci/driver_override"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	chroot /host /bin/bash -c "echo $pci > /sys/bus/pci/drivers/iavf/bind"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	chroot /host /bin/bash -c "echo  > /sys/bus/pci/devices/$pci/driver_override"
	if [ ! $? -eq 0 ]; then
		let ret++
	fi
	echo $ret
}

while getopts buh FLAG; do
   case $FLAG in

   b) Bind="yes" ;;
   u) unBind="yes" ;;
   h) usage ;;
   *) usage ;;
   esac
done

chroot /host /bin/bash -c "modprobe vfio-pci"

if [ "$Bind" == "yes" ]; then
	for i in `ls /sys/class/net`
	do
		# Skip non physical devices
		if [ ! -e /sys/class/net/$i/device ]; then
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
	
		# Skip interface doesn't have vendor id
		if [ ! -e /sys/class/net/$i/device/vendor ]; then
			continue
		fi
	
		# Skip interface not from vendor id
		if [ "$(cat /sys/class/net/$i/device/vendor)" != "0x8086" ]; then
			continue
		fi
	
		pciAddr=`ls -al /sys/class/net/$i/device | awk -F/ '{ print $NF }'`
	
		# Skip interface not vf
		if [ ! -e /sys/bus/pci/devices/$pciAddr/physfn ]; then
			continue
		fi
	
		# Bind VF num to DPDK
		res=$(BindVFToDPDK $pciAddr)
		if [ $res != 0 ]; then
			let failCount++
		fi
	done

	if [ $failCount == 0 ]; then
		echo "successfully bind vfs to vfio-pci driver"
		exit
	else
		echo "failed to bind vfs to vfio-pci driver, exiting"
		exit 1
	fi
fi

if [ "$unBind" == "yes" ]; then

	for i in `ls /sys/bus/pci/devices`
	do
		if [ ! -e /sys/bus/pci/devices/$i/driver ]; then
			continue
		fi

		if [ ! -e /sys/bus/pci/devices/$i/class ]; then
			continue
		fi

		if [ $(cat /sys/bus/pci/devices/$i/class) != "0x020000" ]; then
			continue
		fi

		if [ $(ls -al /sys/bus/pci/devices/$i/driver | awk -F/ '{ print $NF }') == "vfio-pci" ]; then
			res=$(unBindVF $i)
			if [ $res != 0 ]; then
				let failCount++
			fi
		fi
	done

	if [ $failCount == 0 ]; then
		echo "successfully unbind vfs from vfio-pci driver"
		exit
	else
		echo "failed to unbind vfs from vfio-pci driver, exiting"
		exit 1
	fi
fi
