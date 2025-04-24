export default function sortPagesByRank(pages) {
  const integrationsRanked = [];

  Object.keys(pages).forEach((id) => {
    const page = pages[id];

    // Count unique tutorial hosts
    let tutorialsCount = 0;
    if (Array.isArray(page.tutorials)) {
      const hosts = page.tutorials
        .map((url) => {
          try {
            return new URL(url).host;
          } catch {
            return null;
          }
        })
        .filter(Boolean);
      tutorialsCount = new Set(hosts).size;
    }

    // Count videos
    let videosCount = 0;
    if (Array.isArray(page.videos)) {
      videosCount = page.videos.length;
    }

    // Count unique blog hosts
    let blogsCount = 0;
    if (Array.isArray(page.blogs)) {
      const hosts = page.blogs
        .map((url) => {
          try {
            return new URL(url).host;
          } catch {
            return null;
          }
        })
        .filter(Boolean);
      blogsCount = new Set(hosts).size;
    }

    // Count code links
    let codeCount = 0;
    if (Array.isArray(page.code)) {
      codeCount = page.code.length;
    }

    // Final rank
    const rank = tutorialsCount + videosCount + blogsCount + codeCount;

    integrationsRanked.push({ id, rank });
  });

  // Sort descending by rank
  integrationsRanked.sort((a, b) => b.rank - a.rank);

  return integrationsRanked.map((entry) => entry.id);
}
