#! /bin/bash --
#
# download.sh: downloader for StaticPython which autodetects the OS
# by pts@fazekas.hu at Mon May 23 16:06:56 CEST 2011
#
WHICH="python2.7-static"
URL="http://pts-mini-gpl.googlecode.com/svn/trunk/staticpython/release/$WHICH"
if type -p wgetz >/dev/null 2>&1; then
wget -O "$WHICH.download" "$URL" || exit 2
else
echo "info: downloading: $URL"
curl -o "$WHICH.download" "$URL" || exit 2
fi

chmod +x "$WHICH.download"
mv "$WHICH.download" "$WHICH"
echo "info: download OK, run with: ./$WHICH"
