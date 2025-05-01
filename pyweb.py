import subprocess
import threading
import time
from pywebio import start_server
from pywebio.session import run_js
from pywebio.output import put_text
from pywebio.input import input

qemu_process = None
output_buffer = []

def read_qemu_output():
    global output_buffer
    while True:
        if qemu_process is not None:
            try:
                output = qemu_process.stdout.readline()
                if output:
                    for code in ['\033[32m', '\033[0m', '\033[31m', '\033[34m', '\033[36m', '\033[33m', '\033[35m', '\033[37m', '\033[2J', '\033[1;1H', '\033[1;34m', '\033[1;32m']:
                        output = output.replace(code, '')
                    output_buffer.append(output)
            except UnicodeDecodeError:
                pass
        time.sleep(0.01)

def start_qemu():
    global qemu_process
    qemu_process = subprocess.Popen(
        ['C:\Program Files\qemu\qemu-system-x86_64', '-kernel', 'bootable/boot/vmlinuz-1', 
         '-initrd', 'initrd-1.0.img', '-m', '2048', 
         '-nographic', '-append', 'console=ttyS0'],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        stdin=subprocess.PIPE,
        text=True
    )
    threading.Thread(target=read_qemu_output, daemon=True).start()

def send_input_to_qemu(user_input):
    if not "CtrlC" in user_input:
        qemu_process.stdin.write(user_input + '\n')
        qemu_process.stdin.flush()
    else:
        qemu_process.stdin.write('\x03')
        qemu_process.stdin.flush()

def to_bottom():
    run_js('''window.scrollTo({
                    top: document.body.scrollHeight
                });
            ''')

def main():
    start_qemu()
    
    while True:
        if output_buffer:
            put_text(''.join(output_buffer))
            output_buffer.clear()
            to_bottom()
        
        user_input = input()
        if user_input:
            send_input_to_qemu(user_input)
            to_bottom()

if __name__ == '__main__':
    start_server(main, port=8080)

