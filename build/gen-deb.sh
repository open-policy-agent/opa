set -e

if [ ! -f "opa_linux_amd64" ]; then
  echo "ERROR: File 'opa_linux_amd64' not found. First build it 'make build-linux'."
  exit 1
fi

if [ ! -d "man" ]; then
  echo "ERROR: The man pages not found. First build them 'make man'."
  exit 1
fi

if [ -z "$VERSION" ]; then
  echo "ERROR: Need to have VERSION environment variable set."
  exit 1
fi

if ! command -v dpkg-deb >/dev/null 2>/dev/null; then
  echo "ERROR: 'dpkg-deb' was not found and is required."
  exit 1
fi

rm -rf tmp_working_dir_opa
mkdir tmp_working_dir_opa
cd tmp_working_dir_opa

mkdir -p ./usr/bin/
cp ../opa_linux_amd64 ./usr/bin/opa
chmod +x ./usr/bin/opa

mkdir -p ./usr/share/man/man1/
for MAN_PAGE in ../man/*.1; do
  gzip --keep --best $MAN_PAGE
done
mv ../man/*.1.gz ./usr/share/man/man1/

echo "Package: opa" > control
echo "Version: $VERSION" >> control
cat << 'EOF' >> control
Architecture: amd64
Maintainer: OPA Maintainers <cncf-opa-maintainers@lists.cncf.io>
Depends: libc6 (>= 2.2.5)
Section: admin
Priority: optional
Homepage: https://openpolicyagent.org
Description: An open source, general-purpose policy engine.
 The Open Policy Agent (OPA) is an open source, general-purpose
 policy engine that enables unified, context-aware policy
 enforcement across the entire stack.
EOF

md5sum ../opa_linux_amd64 > md5sums

mkdir -p opa_$VERSION/DEBIAN
mv control opa_$VERSION/DEBIAN/control
mv md5sums opa_$VERSION/DEBIAN/md5sums
mv ./usr opa_$VERSION/usr

dpkg-deb -b opa_$VERSION/

mv opa_$VERSION.deb ../
cd ..
rm -rf tmp_working_dir_opa
