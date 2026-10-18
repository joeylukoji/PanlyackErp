(() => {
  const button = document.getElementById("sidebar-collapse");
  if (!button) return;

  const preferenceKey = "panlyack.sidebar.collapsed";
  const apply = (collapsed) => {
    document.documentElement.dataset.sidebarCollapsed = String(collapsed);
    button.setAttribute("aria-expanded", String(!collapsed));
    const label = collapsed ? button.dataset.translateExpand : button.dataset.translateCollapse;
    button.setAttribute("aria-label", label);
    button.title = label;
  };

  let collapsed = false;
  try {
    collapsed = localStorage.getItem(preferenceKey) === "true";
  } catch {
    // Storage may be unavailable; the control still works for this page.
  }
  apply(collapsed);
  button.hidden = false;
  button.addEventListener("click", () => {
    collapsed = !collapsed;
    apply(collapsed);
    try {
      localStorage.setItem(preferenceKey, String(collapsed));
    } catch {
      // Keep navigation usable even when the preference cannot be persisted.
    }
  });
})();
