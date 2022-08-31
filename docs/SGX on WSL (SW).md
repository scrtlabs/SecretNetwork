## How to Run SGX (Software mode) in WSL

For developers that use WSL, using SGX apps is quite annoying. With the default kernel being 5.10 that means there is no support for SGX. 
You can still run SGX in software mode, but only inside a docker container. Which is fine for some use cases, but annoying for others.

To solve this annoyance, you can use a custom Kernel for WSL2. Luckily for us, there is already a release of the 5.15 Kernel, so we don't even have to compile too much.

Todo: Try to compile sgx driver and include it

### 1. Install dependencies

`sudo apt install build-essential flex bison dwarves libssl-dev libelf-dev`

### 2. Clone the WSL2 Repo

`git clone https://github.com/microsoft/WSL2-Linux-Kernel.git --depth 1`

The default branch is on 5.15 already, so no need to switch.

### 3. Copy the .config file

`cp Microsoft/config-wsl .config`

### 4. Compile Kernel

`make` (add `-j 10` with number of cores to speed things up)

Note: I had to set `CONFIG_DEBUG_INFO_BTF=n` in `.config` for it to compile properly

### 5. Copy Kernel to your User folder

`cp arch/x86/boot/bzImage /mnt/c/Users/<UserName>/kernel/`

### 6. Exit WSL and shut it down

`wsl --shutdown`

### 7. Create a .wslconfig file and point it to the kernel

```
[wsl2]
kernel=C:\\Users\\<UserName>\\kernel\\bzImage
```

### 8. Restart WSL and verify that you are running the new Kernel

`uname -a`

And you should see:

`Linux DESKTOP-0H1GO6O 5.15.57.1-microsoft-standard-WSL2+ #2 SMP Thu Aug 18 12:25:48 IDT 2022 x86_64 x86_64 x86_64 GNU/Linux`
