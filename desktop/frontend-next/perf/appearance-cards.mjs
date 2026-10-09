export function appearanceCards(doc) {
  const view = doc.defaultView, advanced = doc.querySelector("details:has(#set-font)"), regular = doc.querySelector("#set-size");
  if (!advanced || !regular) return { pass: false, error: "Appearance cards missing" };
  advanced.open = true;
  const read = (el) => { const css = view.getComputedStyle(el); return { surface: css.backgroundColor, padding: css.padding, radius: css.borderRadius }; };
  const expected = read(regular), heading = view.getComputedStyle(regular.querySelector("h3")).fontSize;
  const cards = [...advanced.querySelectorAll(":scope > summary, :scope > .grp")].map((el) => {
    const actual = read(el), title = el.querySelector(".grp-hd");
    const size = title ? view.getComputedStyle(title.querySelector("h3") ?? title).fontSize : "missing";
    return { name: el.id || "disclosure", ...actual, heading: size, pass: JSON.stringify(actual) === JSON.stringify(expected) && size === heading };
  });
  return { width: view.innerWidth, theme: doc.documentElement.dataset.theme, expected, heading, cards, pass: cards.length === 5 && cards.every((c) => c.pass) && parseFloat(expected.radius) > 0 && parseFloat(expected.padding) > 0 };
}
