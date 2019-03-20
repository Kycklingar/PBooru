# The Permanent Booru
The Permanent Booru is an image board web server which takes leverage of the decentralization of IPFS to distribute its content.

### Requirements
[Golang](https://golang.org)

[Ipfs](https://ipfs.io)

[Postgresql](https://postgresql.org)

[ImageMagick](https://imagemagick.org)

#### Optional
[FFmpeg](https://ffmpeg.org/) For video thumbnails

[Mupdf](https://mupdf.com) For pdf and epub thumbnails

[Gnome-epub-thumbnailer](https://github.com/GNOME/gnome-epub-thumbnailer) For mobi thumbnails

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
