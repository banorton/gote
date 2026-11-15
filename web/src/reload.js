const url = document.body.dataset.events;
if (url) new EventSource(url).addEventListener("reload", () => location.reload());
