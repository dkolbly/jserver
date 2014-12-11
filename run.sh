
# daemon mode:
# docker run -d -e VIRTUAL_HOST=jason.rscheme.org -p 80 -v ~/p/jason/www:/www -v ~/p/jason/www-edit:/edit jserver bin/jserver

docker run --rm -e VIRTUAL_HOST=localhost -p 8302:80 -v $PWD/www:/www -v $PWD/www-edit/ace-builds:/ace -v $PWD/edit:/edit jserver bin/jserver -c /go/src/github.com/dkolbly/jserver/config.json
