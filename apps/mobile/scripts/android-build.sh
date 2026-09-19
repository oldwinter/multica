#!/usr/bin/env bash
set -euo pipefail

MOBILE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$MOBILE_DIR"

# Standalone builds must name their server explicitly. Do not let Expo's
# production dotenv file silently turn a self-hosted build into a cloud client.
export EXPO_NO_DOTENV=1
export APP_ENV=${APP_ENV:-production}
export CI=1
node -e '
for (const key of ["EXPO_PUBLIC_API_URL", "EXPO_PUBLIC_WEB_URL"]) {
  let url;
  try { url = new URL(process.env[key]); } catch { throw new Error(`${key} must be an absolute http(s) URL reachable from your phone`); }
  if (!["https:", "http:"].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error(`${key} must be an http(s) URL without credentials, query or fragment`);
  }
}
const version = Number(process.env.ANDROID_VERSION_CODE || 1);
if (!Number.isSafeInteger(version) || version < 1 || version > 2100000000) throw new Error("ANDROID_VERSION_CODE must be a positive Android version code");
'

: "${ANDROID_HOME:=${ANDROID_SDK_ROOT:-}}"
if [[ -z "$ANDROID_HOME" && -d "$HOME/.local/share/multica-android/sdk" ]]; then
  ANDROID_HOME="$HOME/.local/share/multica-android/sdk"
fi
if [[ -z "${JAVA_HOME:-}" && -d "$HOME/.local/share/multica-android/jdk" ]]; then
  export JAVA_HOME="$HOME/.local/share/multica-android/jdk"
fi
if [[ -z "$ANDROID_HOME" || ! -d "$ANDROID_HOME/build-tools" ]]; then
  echo 'Set ANDROID_HOME to an Android SDK with build-tools installed.' >&2
  exit 1
fi
export ANDROID_HOME
if [[ -n "${JAVA_HOME:-}" ]]; then export PATH="$JAVA_HOME/bin:$PATH"; fi
command -v java >/dev/null || { echo 'Install JDK 21 and set JAVA_HOME.' >&2; exit 1; }

# Expo 57 defaults to recreating native folders; preserve the build cache.
pnpm exec expo prebuild --platform android --no-install --no-clean
(
  cd android
  # Environment variables are not Gradle inputs: explicitly refresh the JS
  # bundle when rebuilding against another server, retaining native caches.
  ./gradlew :app:createBundleReleaseJsAndAssets --rerun :app:assembleRelease \
    --no-daemon --max-workers="${ANDROID_BUILD_WORKERS:-2}" \
    "-PreactNativeArchitectures=${ANDROID_ARCHITECTURES:-arm64-v8a}"
)

# Keep a private, persistent signing identity outside generated android/.
# Re-signing the aligned Gradle APK avoids patching Expo's generated Gradle
# files, and replacing Expo's shared debug key makes subsequent installs safe.
package_id=$(pnpm exec expo config --type public --json | node -e 'let s=""; process.stdin.on("data", x=>s+=x).on("end", ()=>process.stdout.write(JSON.parse(s).android.package))')
signing_dir="${MULTICA_ANDROID_SIGNING_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/multica/android-signing/$package_id}"
umask 077
mkdir -p "$signing_dir" dist
keystore="$signing_dir/release.p12"
password_file="$signing_dir/password"
if [[ ! -f "$keystore" ]]; then
  if [[ ! -f "$password_file" ]]; then
    node -e 'process.stdout.write(require("node:crypto").randomBytes(32).toString("hex"))' > "$password_file"
  fi
  keytool -genkeypair -keystore "$keystore" -storetype PKCS12 \
    -storepass:file "$password_file" -keypass:file "$password_file" \
    -alias multica -keyalg RSA -keysize 3072 -validity 10000 \
    -dname 'CN=Multica Personal Android'
fi
[[ -s "$password_file" ]] || { echo "Missing signing password: $password_file" >&2; exit 1; }
build_tools=$(node -e 'const fs=require("node:fs"),p=require("node:path"),d=p.join(process.env.ANDROID_HOME,"build-tools"); const versions=fs.readdirSync(d).filter(v=>/^\d+\.\d+\.\d+$/.test(v)).sort((a,b)=>a.localeCompare(b,undefined,{numeric:true})); if(!versions.length) throw new Error("No stable Android build-tools installed"); process.stdout.write(p.join(d,versions.at(-1)))')
apk="dist/multica-android.apk"
# PKCS12 uses one password for the store and key; apksigner reuses it.
"$build_tools/apksigner" sign --ks "$keystore" --ks-key-alias multica \
  --ks-pass "file:$password_file" \
  --out "$apk" android/app/build/outputs/apk/release/app-release.apk
"$build_tools/apksigner" verify "$apk"
"$build_tools/zipalign" -c -P 16 4 "$apk"
node -e 'const fs=require("node:fs"),crypto=require("node:crypto"),p="dist/multica-android.apk"; fs.writeFileSync(p+".sha256",crypto.createHash("sha256").update(fs.readFileSync(p)).digest("hex")+"  multica-android.apk\n")'
printf '\nAPK: %s/%s\nBack up the private signing directory to keep app updates installable: %s\n' "$MOBILE_DIR" "$apk" "$signing_dir"
