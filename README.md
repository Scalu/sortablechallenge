<p>This is my entry for the sortable coding challenge: https://sortable.com/challenge/.
It's not perfect, I'd like to completely approach the matching in a different manner altogether, but the deadline is coming up and the rewrite could take a bit too long. I might get to it sometime later.
It is written in Go (or Golang if you will), which I have no previous experience with. It was a fun learning experience.</p>

<p>To run this on Ubuntu or other similar linux systems you need to have git and go installed. The following instructions assume to are running on a 64-bit processor, and you only need to copy and paste the commands that are needed. i.e. don't run the command to install git if you already have git installed on your system.</p>

<p><b>To install git, run:</b> sudo apt install git</p>

<p><b>To install go, run:</b> wget https://storage.googleapis.com/golang/go1.8.linux-amd64.tar.gz; sudo tar -C /usr/local -xzf go1.8.linux-amd64.tar.gz; sudo echo "PATH=\$PATH:/usr/local/go/bin" >> ~/.profile; export PATH=$PATH:/usr/local/go/bin</p>
<p><b>Alternately, although you may get an older version, you can run:</b> sudo apt install golang-go</p>

<p><b>To clone my repo, run:</b> mkdir -p ~/go/src/github.com/Scalu; cd ~/go/src/github.com/Scalu; git clone https://github.com/Scalu/sortablechallenge.git</p>

<p><b>To build and run my code, run:</b> export GOPATH=~/go; cd ~/go/src/github.com/Scalu/sortablechallenge; go build; ./sortablechallenge</p>
