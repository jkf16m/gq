# Maintainer: Your Name <your@email.com>
pkgname=gq-git
pkgver=0.1.0
pkgrel=1
pkgdesc="A git-like CLI tool"
arch=('x86_64')
url="https://github.com/yourusername/gq"
license=('MIT')
depends=()
makedepends=('go')
source=("git+file:///home/klip/projects/gq")
sha256sums=('SKIP')

build() {
  cd "$srcdir"
  go run build.go build
}

package() {
  cd "$srcdir"
  install -Dm755 gq "$pkgdir/usr/bin/gq"
}
