BINARIES := cat chmod chown cp echo rm touch yes sh whoami clear mkdir ls neofetch uname

all: build
	echo ""

build_run: build qemu
	echo ""

iso: build
	cp initrd-1.0.img bootable/boot/
	grub-mkrescue -o bootable.iso bootable

build: clean
	for f in $(wildcard *.go); do echo $$f; GOOS=linux go build -ldflags="-s -w" $$f; done
	cp $(BINARIES) linux/bin
	rm -f $(BINARIES)
	for f in $(BINARIES); do chmod 777 linux/bin/$$f; done
	echo $(BINARIES) > linux/usr/possibilities
	sh make_cpio.sh

clean:
	for f in $(BINARIES); do rm -f linux/bin/$$f; done
	echo "0" > initrd-1.0.img
	echo "0" > bootable/boot/initrd-1.0.img
	echo "0" > linux/usr/possibilities
	echo "0" > bootable.iso
	rm initrd-1.0.img
	rm bootable/boot/initrd-1.0.img
	rm linux/usr/possibilities
	rm bootable.iso

qemu:
	qemu-system-x86_64 -kernel bootable/boot/vmlinuz-1 -initrd initrd-1.0.img -m 2048
qemu-console:
	qemu-system-x86_64 -kernel bootable/boot/vmlinuz-1 -initrd initrd-1.0.img -m 2048 -nographic -append 'console=ttyS0'