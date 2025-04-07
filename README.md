# Coreutils on Go

Multi-Requirements: ```sudo apt install --yes make build-essential bc bison flex libssl-dev libelf-dev wget cpio fdisk dosfstools qemu-system-x86 golang```.

Compile: ```make```;
Clean: ```make clean```;
Compile and run in qemu: ```build_run```
Run in qemu: ```make qemu```
Make bootable iso (grub is needed): ```make iso``