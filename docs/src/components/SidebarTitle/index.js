export function SidebarTitle(title, options = {}) {
  const { space = true } = options;

  return {
    type: "html",
    value: `
      <div style="color: var(--ifm-color-emphasis-600); font-size: 1rem;${space ? " margin-top: 0.4rem;" : ""}">
        ${title}
      </div>
    `,
  };
}
