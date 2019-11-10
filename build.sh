#!/usr/bin/env bash

package="github.com/foae/gorgonzola"
package_name="gorgonzola"
package_split=(${package//\// })
platforms=("linux/amd64" "darwin/amd64")

cd ./cmd/$package_name

for platform in "${platforms[@]}"; do
  platform_split=(${platform//\// })
  GOOS=${platform_split[0]}
  GOARCH=${platform_split[1]}
  output_name=$package_name'-'$GOOS'-'$GOARCH

  if [ $GOOS = "windows" ]; then
    output_name+=".exe"
  fi

  echo -e "\e[1;34mBuilding $output_name... \e[0m"

  env CGO_ENABLED=1 GOOS=$GOOS GOARCH=$GOARCH go build -a -o ../../bin/$output_name
  if [ $? -ne 0 ]; then
    echo -e "\e[1;31mAn error has occurred while building ($output_name). No build will be made available for this platform. \e[0m"
    continue
  fi

  chmod +x ../../bin/$output_name
  echo -e "\e[1;32mFinished building ($package_name) for ($platform) in ($(pwd)) \e[0m"
done
