export default function getLogoAsset(key) {
  const logos = require.context("@site/static/img/ecosystem-entry-logos");
  const allKeys = logos.keys();
  const prefix = `./${key}.`;
  const defaultPng = "./default.png";

  const foundFilename = allKeys.find(k => k.startsWith(prefix));

  let filenameToLoad = null;

  if (foundFilename) {
    filenameToLoad = foundFilename;
  } else if (key !== "default" && allKeys.includes(defaultPng)) {
    filenameToLoad = defaultPng;
  } else if (key === "default" && allKeys.includes(defaultPng)) {
    filenameToLoad = defaultPng;
  }

  if (filenameToLoad) {
    try {
      const module = logos(filenameToLoad);
      return module.default || module;
    } catch (error) {
      console.error(`Error loading logo asset '${filenameToLoad}':`, error);
      if (filenameToLoad !== defaultPng && allKeys.includes(defaultPng)) {
        try {
          const defaultModule = logos(defaultPng);
          return defaultModule.default || defaultModule;
        } catch (defaultError) {
          console.error(`Error loading fallback default logo '${defaultPng}':`, defaultError);
          return null;
        }
      }

      return null;
    }
  } else {
    console.warn(`Logo key '${key}' not found, and default.png is also missing or not in context.`);

    return null;
  }
}
