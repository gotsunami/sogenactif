sogenactif
==========

Provides support for the online payment solution provided by la Société Générale.

Please note that this library depends on two closed source binaries `request` and `response` (provided 
by the Sogenactif platform) located in the `lib/` directory. They handle certificates, signing operations 
and provide data in a proprietary format to a remote secure payment server...

For the moment, only binaries for linux/386 and linux/amd64 are available in this repository.

Installation
------------

Use the `sogen` tool to quickly test a configuration:

    go get github.com/matm/sogenactif/sogen
    
which has the following usage:

    Usage: ./sogen [options] settings.conf 

    Options:
      -p="6060": http server listening port
      -t=1: transaction amount
  
Run a demo
----------

You can run a demo and test a fake transaction of 5 EUR with:

    ./sogen -t=5 conf/demo.cfg
    
API doc
-------

More docs and API documentation available on godoc:

http://godoc.org/github.com/matm/sogenactif


