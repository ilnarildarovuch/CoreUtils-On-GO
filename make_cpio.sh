echo 0 > initrd-1.0.img
rm initrd-1.0.img
cd linux
find . | cpio -o -H newc > ../initrd-1.0.img
cd ..