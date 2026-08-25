# Maintainer: Your Name <your@email.com>
pkgname=gq
pkgver=0.1.0
pkgrel=1
pkgdesc="A git-like CLI tool"
arch=('x86_64')
license=('MIT')
depends=()
makedepends=('go')
source=("$pkgname-$pkgver::git+file://$startdir")
sha256sums=('SKIP')

build() {
  cd "$srcdir/$pkgname-$pkgver"
  mkdir -p bin
  go build -o bin/gq .
}

package() {
  cd "$srcdir/$pkgname-$pkgver"
  install -Dm755 bin/gq "$pkgdir/usr/bin/gq"
}
