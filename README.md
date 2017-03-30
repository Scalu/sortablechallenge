<p>This is my entry for the sortable coding challenge: https://sortable.com/challenge/.
It's not perfect, I'd like to completely approach the matching in a different manner altogether, but the deadline is coming up and the rewrite could take a bit too long. I might get to it sometime later.
It is written in Go (or Golang if you will), which I have no previous experience with. It was a fun learning experience.</p>

<p>To run this on Ubuntu or other similar linux systems you need to have git and go installed</p>

<p>To install git, run: sudo apt install git</p>

<p>To install go, run: wget https://storage.googleapis.com/golang/go1.8.linux-amd64.tar.gz; sudo tar -C /usr/local -xzf go1.8.linux-amd64.tar.gz; sudo echo "PATH=\$PATH:/usr/local/go/bin" >> ~/.profile; export PATH=$PATH:/usr/local/go/bin</p>
<p>Alternately, although you may get an older version, you can run: sudo apt install golang-go</p>

<p>To clone my repo, run: mkdir -p ~/go/src/github.com/Scalu; cd ~/go/src/github.com/Scalu; git clone https://github.com/Scalu/sortablechallenge.git</p>

<p>To build and run my code, run: export GOPATH=~/go; cd ~/go/src/github.com/Scalu/sortablechallenge; go build; ./sortablechallenge</p>
