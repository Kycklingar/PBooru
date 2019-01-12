# The Permanent Booru

### Requirements
[Golang](https://golang.org)
[Ipfs](https://ipfs.io)
[Postgresql](https://postgresql.org)
[ImageMagick](https://imagemagick.org)

### Installing
Go get and build PBooru
```
go get -u -d github.com/kycklingar/PBooru
cd $GOPATH/src/github.com/kycklingar/PBooru
./build.sh
cp out ~/pbooru
```

Go get and install Ipfs
```
go get -u -d github.com/ipfs/go-ipfs
cd $GOPATH/src/github.com/ipfs/go-ipfs
make install
```

Download and setup Postgresql.
Create a user and a database for PBooru.

### Running
```
cd ~/pbooru
./pbooru -init-cfg
```
Edit the config.cfg file, mainly change the DBCfg.ConnectionString and HTTPAddress.
Run ./pbooru
