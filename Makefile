

all: cogsd cogs

clean:
	-rm -rf bin

bin:
	mkdir -p bin

cogsd: bin cogsd/*.go
	cd cogsd && go build -o ../bin/cogsd 

cogs: bin cogs/*.go
	cd cogs && go build -o ../bin/cogs

