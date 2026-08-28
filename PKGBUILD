# Maintainer: klip
pkgname=gq
pkgver=0.1.0
pkgrel=1
pkgdesc='A terminal AI agent with explicit shell-command approval'
arch=('x86_64')
url='https://github.com/jkf16m/gq'
license=('MIT')
depends=()
makedepends=('go')
# Build the checkout being packaged. This keeps `makepkg -si` useful during
# development and ensures the installed binary matches the current source.
source=()
sha256sums=()

build() {
  cd "$startdir"
  go build -trimpath -buildmode=pie -ldflags='-s -w' -o "$srcdir/gq" .
}

check() {
  cd "$startdir"
  go test . ./cmd ./config ./project ./session
}

package() {
  install -Dm755 "$srcdir/gq" "$pkgdir/usr/bin/gq"
  install -Dm644 "$startdir/LICENSE" "$pkgdir/usr/share/licenses/$pkgname/LICENSE"
}
