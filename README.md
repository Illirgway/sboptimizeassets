[Starbound](https://starbounder.org/Starbound) Assets Optimizer
===============================================================

Optimizes only PNG files (lossless obfuscate) for now.

### WARNING
It does not create backups and rewrites files in-place!


## Usage

```sh
# require go v1.20+

mkdir -p "${GOPATH}/src/github.com/Illirgway/"
cd "${GOPATH}/src/github.com/Illirgway/"

git clone --depth 1 --branch master https://github.com/Illirgway/sboptimizeassets.git

cd sboptimizeassets
mkdir bin

go build -v -ldflags "-s -w" -o bin/sboptimizer .

# check
bin/sboptimizer -h

# run
bin/sboptimizer --dir "/abs/path/to/optimizing/mod/root/dir"

# or with rel path
cd "/starbound/mods/dir"
/path/to/bin/sboptimizer --dir "my_cool_mod"
```

### TODO
* careful specialized optimization for gray16, rgba64, nrgba64

## LICENSE
GNU GPL v3
