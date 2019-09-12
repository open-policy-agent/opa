import path from 'path'
import {constants as fsConsts, promises as promFS} from 'fs'

import fetch from 'node-fetch'

import {ChainedError} from '../errors'
import {handleLater, println} from '../helpers'
import {OPA_CACHE_PATH, OPA_EDGE_CACHE_PERIOD, PLATFORMS, VERSION_EDGE, VERSION_LATEST} from '../constants'

const PLATFORM = (() => { // Current platform, massaged into the format that OPA releases use.
  let p = process.platform
  if (p === 'win32') {
    p = PLATFORMS.WINDOWS
  }
  return p
})()
const OPA_PATH = path.resolve(OPA_CACHE_PATH)

const acquireResults = {} // Cache: object mapping version strings to promises that will resolve to [path, error] where one is undefined.

// Returns a promise that either resolves to an absolute path to a opa binary of that version that is executable by the current process on the current platform or rejects with a user-friendly error.
export async function acquireOPAVersion(version) {
  // Synchronously (no awaits, within this function) create and cache the acquirer if needed, prevents other calls to acquireOPAVersion that are running around the same time from creating duplicate acquirers.
  if (!acquireResults[version]) {
    acquireResults[version] = handleLater(createAcquirer(version))
  }

  // Actually wait for the acquirer to do it's job, handle the result.
  const [path, err] = await acquireResults[version] // The same handleLater promise may be awaited multiple times before and/or after it resolves, this isn't a problem.
  if (err) {
    throw err
  } else {
    return path
  }
}

// Returns an async function that either resolves to a path or rejects with a user-friendly error.
// WARNING: This function and it's resulting acquirer should only get called once per version.
function createAcquirer(version) {
  return async () => {
    const path = pathToVersion(version)
    if (await needsDownloading(version, path)) { // This does not protect against run conditions within a single preprocessing script run, simply allows (long-term) caching.

      println(`Locating OPA version ${version}...`)
      const assetURL = await getAssetURL(version)

      let file
      try {
        println(`Downloading OPA version ${version} from ${assetURL}...`)
        await promFS.mkdir(OPA_PATH, {recursive: true}) // recursive also prevents error if the folder already exists
        file = await promFS.open(path, 'w', 0o744) // create file with permissions rwx for user, r for group, other
        await promFS.writeFile(file, await (await fetch(assetURL)).buffer())
        await file.close()
      } catch (e) {
        if (file) { // Close the file if fetching/writing fails
          await file.close()
        }
        throw new ChainedError(`unable to download the OPA version ${version} for ${PLATFORM} from ${assetURL}`, e)
      }
    }
    return path
  }
}

// The absolute path where an OPA copy should be downloaded to if it doesn't exist there already.
function pathToVersion(version) {
  return path.resolve(OPA_PATH, `${version}-${PLATFORM}${PLATFORM === PLATFORMS.WINDOWS ? '.exe' : ''}`)
}

// Gets the URL that can be used to download a given version of OPA for the current platform. May error with a user-friendly message.
async function getAssetURL(version) {
  // Edge is built by travis automatically
  if (version === VERSION_EDGE) {
    return `https://opa-releases.s3.amazonaws.com/edge/opa_${PLATFORM}_amd64${PLATFORM === PLATFORMS.WINDOWS ? '.exe' : ''}`
  }

  // Releases are on GitHub
  let releaseURL;
  if (version === VERSION_LATEST) {
    releaseURL = `https://api.github.com/repos/open-policy-agent/opa/releases/latest`
  } else {
    releaseURL = `https://api.github.com/repos/open-policy-agent/opa/releases/tags/${version}`;
  }

  try {
    const release = await (await fetch(releaseURL)).json()
    for (let asset of Object.values(release.assets)) {
      if (asset.name.indexOf(PLATFORM) != -1) {
        if (asset.browser_download_url) { // If it's actually set, return it
          return asset.browser_download_url
        }
        // Implicit else, throw error below.
      }
    }
  } catch (e) {
    throw new ChainedError(`error occured while getting the OPA release asset URL for ${version} on ${PLATFORM} from ${releaseURL}`, e)
  }
  throw new Error(`unable to get the OPA release asset URL for ${version} on ${PLATFORM} from ${releaseURL}`)
}

// Determines, based on the desired version and the path to where it should be downloaded, whether it needs to be downloaded. Will not throw an error.
async function needsDownloading(version, path) {
  try {
    await promFS.access(path, fsConsts.X_OK) // X_OK verifies that permissions-wise it's executable
  } catch (e) {
    return true // Can't be accessed, download it
  }

  if (version === VERSION_EDGE) {
    // Invalidate the edge cache based on when it was downloaded
    try {
      return new Date().getTime() - (await promFS.stat(path)).mtimeMs > OPA_EDGE_CACHE_PERIOD
    } catch (e) {
      return true // That's odd, shouldn't error given that it can be accessed. Anyway, might as well try redownloading it.
    }
  } else {
    // Don't unnecessarily redownload released versions
    return false
  }

}
