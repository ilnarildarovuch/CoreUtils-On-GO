BINARIES := cat chmod chown cp echo rm touch yes simple_shell whoami clear mkdir

all: build
	echo ""

build_run: build qemu
	echo ""

iso: build
	cp initrd-1.0.img bootable/boot/
	grub-mkrescue -o bootable.iso bootable

build: clean
	for f in $(wildcard *.go); do echo $$f; GOOS=linux GOARCH=386 go build -ldflags="-s -w" $$f; done
	cp $(BINARIES) linux/bin
	rm -f $(BINARIES)
	for f in $(BINARIES); do chmod 777 linux/bin/$$f; done
	echo $(BINARIES) > linux/usr/possibilities
	sh make_cpio.sh

clean:
	rm -R linux/bin
	mkdir linux/bin
	echo "0" > initrd-1.0.img
	echo "0" > bootable/boot/initrd-1.0.img
	echo "0" > linux/usr/possibilities
	echo "0" > bootable.iso
	rm initrd-1.0.img
	rm bootable/boot/initrd-1.0.img
	rm linux/usr/possibilities
	rm bootable.iso

qemu:
	qemu-system-x86_64 -kernel vmlinuz-1 -initrd initrd-1.0.img -m 2048 -vga std -device virtio-keyboard-pci -device virtio-mouse-pci -device virtio-gpu-pci