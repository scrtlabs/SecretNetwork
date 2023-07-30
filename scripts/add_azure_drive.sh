#!/bin/bash

# Find the new drive without partitions
new_drive=$(lsblk -o NAME,SIZE,TYPE,MOUNTPOINT -r -n -p | grep -w disk | awk '{print $1}' | while read -r drive; do if [[ $(lsblk -o NAME -n -r -p "$drive") == "$drive" ]]; then echo "$drive"; fi; done)

if [ -z "$new_drive" ]; then
  echo "No new drive found. Exiting."
  exit 1
fi

echo "New drive found: $new_drive"

# Partition and format the new drive
echo "Partitioning and formatting the new drive..."
sudo parted "$new_drive" --script mklabel gpt mkpart xfspart xfs 0% 100%
sudo mkfs.xfs "${new_drive}1"
sudo partprobe "${new_drive}1"

# Mount the new drive
echo "Creating /datadrive directory and mounting the new drive..."
sudo mkdir /datadrive
sudo mount "${new_drive}1" /datadrive

# Find UUID of the new partition
uuid=$(sudo blkid | grep "${new_drive}1" | awk -F '"' '{print $2}')

# Add the new drive to fstab
echo "Adding the new drive to /etc/fstab..."
fstab_backup="/etc/fstab.$(date +%Y%m%d%H%M%S).bak"
sudo cp /etc/fstab $fstab_backup
echo "UUID=$uuid   /datadrive   xfs   defaults,nofail   1   2" | sudo tee -a /etc/fstab

echo "Done. The new drive has been partitioned, formatted, and added to  /etc/fstab."
