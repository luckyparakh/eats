Create Ubuntu VM using Virtual Box then connect it on Host VSCode if needed:

To connect Visual Studio Code on your host machine to an Ubuntu guest VM inside VirtualBox, use the VS Code Remote - SSH extension. This allows you to edit files and run commands on your VM seamlessly from your host machine. [1, 2, 3, 4, 5]  
Follow this four-step process to get everything running. 
1. Configure the VirtualBox Network [6]  
You must configure the VM network settings so the host machine can reach it via an IP address. 

1. Shut down your Ubuntu VM. 
2. Open VirtualBox, select your Ubuntu VM, and click Settings. 
3. Navigate to Network $\rightarrow$ Adapter 2. 
4. Check Enable Network Adapter. 
5. Set the Attached to: dropdown to Host-only Adapter. 
6. Click OK and start your VM. [7, 9, 10, 11, 12]  

2. Enable SSH Server on Ubuntu [13]  
Your Ubuntu VM needs an active SSH server to accept incoming connections from VS Code. 

1. Open the terminal inside your Ubuntu VM. 
2. Update your package list and install the OpenSSH server: sudo apt update && sudo apt install openssh-server -y
3. Verify that the SSH service is actively running: sudo systemctl status ssh
4. Find the IP address assigned by the Host-only network by running: ip a
5. Look for the adapter prefixed with  or  (usually under Adapter 2, like ). Note down the IPv4 address (e.g., 192.168.56.101). [7, 14, 15, 16, 17]  

3. Install the Remote - SSH Extension in VS Code 

1. Open Visual Studio Code on your host computer. 
2. Click the Extensions icon on the left sidebar (or press  / ). 
3. Search for Remote - SSH and click Install. [2, 19, 20]  

4. Connect VS Code to the Ubuntu VM 

1. In VS Code, click the green Open a Remote Window icon (two arrowheads ) in the bottom-left corner of the window. 
2. Select Remote-SSH: Connect to Host... from the dropdown menu. 
3. Choose Add New SSH Host.... 
4. Input the connection string using your Ubuntu username and the IP address you found in Step 2: 
5. Select the default SSH configuration file to save the target (usually  on Windows). 
6. Click Connect on the pop-up notification, select Linux as the platform, and input your Ubuntu user password when prompted. [2, 19, 20, 21, 22, 23, 24, 25]  

Once connected, the green indicator in the bottom-left corner will display . You can now go to File &gt; Open Folder to browse and edit files stored directly inside your Ubuntu environment. [23, 26, 27]  
If you hit any configuration snags, let me know: 

• What Operating System your host machine runs (Windows, macOS, or Linux)? 
• Any specific error message popping up when VS Code attempts to connect? 

I can give you precise troubleshooting steps or show you how to configure SSH keys so you don't have to type your password every single time. [3, 20]  

AI can make mistakes, so double-check responses

[1] https://www.mantrax.io/use-vscode-to-code-on-remote-machine/
[2] https://code.visualstudio.com/blogs/2019/07/25/remote-ssh
[3] https://www.youtube.com/watch?v=ruScuix-7f0
[4] https://dev.to/dpuig/setting-up-an-ubuntu-dev-environment-with-multipass-and-vs-code-remote-ssh-1709
[5] https://www.databasemart.com/kb/connect-to-linux-server-via-vs-code
[6] https://alexhost.com/faq/configuring-the-network-in-virtualbox/
[7] https://stackoverflow.com/questions/58880989/connecting-visual-studio-code-vscode-to-virtualbox-vm
[8] https://www.bigstack.co/blog/cubecos-create-virtual-machine
[9] https://stackoverflow.com/questions/1261975/addressing-localhost-from-a-virtualbox-virtual-machine
[10] https://blog.hyperiondev.com/post/virtualbox-tutorial/
[11] https://codersports.com/2024/03/28/setting-up-an-ubuntu-virtual-machine-in-virtualbox/
[12] https://superuser.openinfra.org/articles/how-to-set-up-a-virtual-machine-with-virtualbox/
[13] https://www.mantrax.io/use-vscode-to-code-on-remote-machine/
[14] https://intranet.neuro.polymtl.ca/geek-tips/misc/virtual-machines/virtualbox.html
[15] https://nishant-servonode.medium.com/install-virtualbox-in-ubuntu-21-04-20-04-18-04-servo-node-6d2dbce57b72
[16] https://simplificandoredes.com/en/how-to-install-ubuntu-server-on-virtualbox/
[17] https://medium.com/smartcomputing/how-to-set-up-a-local-cloud-vm-with-xubuntu-and-virtualbox-on-windows-8a172ec7e614
[18] https://code.visualstudio.com/docs/remote/ssh
[19] https://www.linuxtek.ca/2021/11/25/setting-up-a-linux-environment-with-virtualbox-and-visual-studio-code/
[20] https://deedeo.hashnode.dev/how-to-connect-virtual-machine-to-vs-code
[21] https://ubuntu.com/blog/how-to-create-a-vscode-linux-remote-environment
[22] https://www.storylane.io/tutorials/how-to-remote-ssh-connect-to-host-in-vs-code
[23] https://www.digitalocean.com/community/tutorials/how-to-use-visual-studio-code-for-remote-development-via-the-remote-ssh-plugin
[24] https://medium.com/@stevernewman/how-to-use-visual-studio-code-on-linux-virtual-server-a4baec51040b
[25] https://dev.to/arabian619/connect-a-vm-with-vs-code-for-remote-development-50n7
[26] https://dev.to/buttonfreak/vscode-server-on-azure-ubuntu-vm-a-step-by-step-guide-38h5
[27] https://networkotaku.wordpress.com/2018/12/10/using-visual-studio-code-for-creating-network-code/

