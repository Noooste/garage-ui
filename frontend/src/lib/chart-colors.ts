// Categorical palette in fixed order, each mode stepped for its own card surface.
// Adjacent pairs stay distinguishable under protan, deutan and tritan vision.
export const categoricalPalette = {
  light: ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#6250d6', '#e34948'],
  dark: ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#008300', '#9085e9', '#e66767'],
};

// Neutral for aggregate slices such as "Other", so they never read as a series.
export const otherColor = {
  light: '#b8b2a7',
  dark: '#5d6378',
};
