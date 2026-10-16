// Only guards unsaved input; document state and rendering remain on the server.
let hasUnsavedReference = false;
document.addEventListener('input', (event) => {
  if (event.target.closest('#reference-form')) hasUnsavedReference = true;
});
document.addEventListener('change', (event) => {
  if (event.target.closest('#reference-form')) hasUnsavedReference = true;
});
document.addEventListener('htmx:confirm', (event) => {
  const element = event.detail.elt;
  const leavesDocument = element.closest('a') || (element.closest('form') && !element.closest('#reference-form'));
  if (hasUnsavedReference && leavesDocument) {
    event.preventDefault();
    if (window.confirm('Quitter sans enregistrer les modifications de cette référence ?')) {
      hasUnsavedReference = false;
      event.detail.issueRequest(true);
    }
  }
});
document.addEventListener('htmx:afterRequest', (event) => {
  if (event.detail.xhr.getResponseHeader('X-Review-Saved') === 'true') hasUnsavedReference = false;
});
window.addEventListener('beforeunload', (event) => {
  if (hasUnsavedReference) {
    event.preventDefault();
    event.returnValue = '';
  }
});
