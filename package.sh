#!/bin/sh
set -e

# compile for version
make
if [ $? -ne 0 ]; then
    echo "make error"
    exit 1
fi

proxy_rproxy_version=`0.0.1`
echo "build version: $proxy_rproxy_version"

# cross_compiles
make -f ./Makefile.cross-compiles

rm -rf ./release/packages
mkdir -p ./release/packages

os_all='linux windows darwin freebsd android'
arch_all='386 amd64 arm arm64 mips64 mips64le mips mipsle riscv64 loong64'
extra_all='_ hf'

cd ./release

for os in $os_all; do
    for arch in $arch_all; do
        for extra in $extra_all; do
            suffix="${os}_${arch}"
            if [ "x${extra}" != x"_" ]; then
                suffix="${os}_${arch}_${extra}"
            fi
            proxy_rproxy_dir_name="proxy_rproxy_${proxy_rproxy_version}_${suffix}"
            proxy_rproxy_path="./packages/proxy_rproxy_${proxy_rproxy_version}_${suffix}"

            if [ "x${os}" = x"windows" ]; then
                if [ ! -f "./proxy_rproxy_${os}_${arch}.exe" ]; then
                    continue
                fi
                mkdir ${proxy_rproxy_path}
                mv ./proxy_rproxy_${os}_${arch}.exe ${proxy_rproxy_path}/proxy_rproxy.exe
            else
                if [ ! -f "./proxy_rproxy_${suffix}" ]; then
                    continue
                fi
                mkdir ${proxy_rproxy_path}
                mv ./proxy_rproxyc_${suffix} ${proxy_rproxy_path}/proxy_rproxy
                mv ./proxy_rproxys_${suffix} ${proxy_rproxy_path}/proxy_rproxy
            fi  
            cp ../LICENSE ${proxy_rproxy_path}

            # packages
            cd ./packages
            if [ "x${os}" = x"windows" ]; then
                zip -rq ${proxy_rproxy_dir_name}.zip ${proxy_rproxy_dir_name}
            else
                tar -zcf ${proxy_rproxy_dir_name}.tar.gz ${proxy_rproxy_dir_name}
            fi  
            cd ..
            rm -rf ${proxy_rproxy_path}
        done
    done
done

cd -