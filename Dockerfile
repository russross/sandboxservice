FROM debian:jessie
MAINTAINER russ@dixie.edu
ENV TZ America/Denver

# install binaries
ADD sandboxservice /usr/local/bin/
ADD bin/sandbox /usr/local/bin/
ADD bin/python2.7-static /usr/local/bin/

RUN useradd -m --uid 1410 student
USER student
WORKDIR /home/student
EXPOSE 8081
CMD ["/usr/local/bin/sandboxservice", ":8081"]

# to build:
#   go build
#   build sandbox and copy to bin/
#   run fetchpython.py in bin/
#
#   docker build -t sandboxservice .
#
# to run, copy sandboxservice.conf to /etc/init and run: sudo service sandboxservice start
