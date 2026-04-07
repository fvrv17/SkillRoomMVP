export const ROOM_LEVELS = ["bronze", "silver", "gold", "platinum"];

export const ROOM_THEMES = {
  classic: {
    id: "classic",
    label: "Classic shell",
    description: "Clean studio box for the default SkillRoom experience.",
    shellAsset: "/room/shell/classic-shell.svg",
    overlayAsset: "",
  },
  holiday: {
    id: "holiday",
    label: "Holiday shell",
    description: "Seasonal overlay layer for limited-time drops and monetized decor packs.",
    shellAsset: "/room/shell/classic-shell.svg",
    overlayAsset: "/room/themes/holiday-overlay.svg",
  },
};

export const ROOM_WINDOW_SCENES = {
  daylight: {
    id: "daylight",
    label: "Daylight",
    asset: "/room/windows/daylight.svg",
  },
  sunset: {
    id: "sunset",
    label: "Sunset",
    asset: "/room/windows/sunset.svg",
  },
  night: {
    id: "night",
    label: "Night",
    asset: "/room/windows/night.svg",
  },
};

export const ROOM_DEFAULT_THEME_ID = "classic";
export const ROOM_DEFAULT_WINDOW_SCENE_ID = "daylight";

function buildItemAssets(code) {
  return Object.fromEntries(ROOM_LEVELS.map((level) => [level, roomAssetPath(code, level)]));
}

export const ROOM_SCENE_ORDER = ["shelf", "trophy_case", "monitor", "desk", "chair", "plant"];

export const ROOM_SCENE_SLOTS = {
  shelf: {
    code: "shelf",
    title: "Shelf / Volume",
    skill: "Volume",
    placeholderTitle: "Shelf placeholder",
    availableLevels: [],
    assets: buildItemAssets("shelf"),
    style: {
      left: "72.2%",
      top: "18.2%",
      width: "17.2%",
      height: "12.8%",
      zIndex: 22,
      transform: "translate3d(0, 0, 0)",
    },
  },
  trophy_case: {
    code: "trophy_case",
    title: "Trophy / Achievements",
    skill: "Achievements",
    placeholderTitle: "Trophy placeholder",
    presentationMode: "achievement_case",
    showLevelBadge: false,
    availableLevels: [],
    assets: buildItemAssets("trophy_case"),
    style: {
      left: "55.8%",
      top: "23.4%",
      width: "12.4%",
      height: "20.8%",
      zIndex: 18,
      transform: "translate3d(0, 0, 0)",
    },
  },
  monitor: {
    code: "monitor",
    title: "Monitor / React",
    skill: "React",
    placeholderTitle: "Monitor placeholder",
    availableLevels: [],
    assets: buildItemAssets("monitor"),
    style: {
      left: "57.8%",
      bottom: "34.5%",
      width: "15.8%",
      height: "13.8%",
      zIndex: 28,
      transform: "translate3d(-50%, 0, 0)",
    },
  },
  desk: {
    code: "desk",
    title: "Desk / JavaScript",
    skill: "JavaScript",
    placeholderTitle: "Desk placeholder",
    availableLevels: [],
    assets: buildItemAssets("desk"),
    style: {
      left: "65.6%",
      bottom: "21%",
      width: "35%",
      height: "17.4%",
      zIndex: 17,
      transform: "translate3d(-50%, 0, 0)",
    },
  },
  chair: {
    code: "chair",
    title: "Chair / Architecture",
    skill: "Architecture",
    placeholderTitle: "Chair placeholder",
    availableLevels: [],
    assets: buildItemAssets("chair"),
    style: {
      left: "41.8%",
      bottom: "19.5%",
      width: "17.4%",
      height: "24.8%",
      zIndex: 24,
      transform: "translate3d(-50%, 0, 0)",
    },
  },
  plant: {
    code: "plant",
    title: "Plant / Consistency",
    skill: "Consistency",
    placeholderTitle: "Plant placeholder",
    availableLevels: [],
    assets: buildItemAssets("plant"),
    style: {
      right: "9.6%",
      bottom: "21.5%",
      width: "10.6%",
      height: "20.8%",
      zIndex: 24,
      transform: "translate3d(0, 0, 0)",
    },
  },
};

export function normalizeRoomLevel(level) {
  const normalized = String(level || "").toLowerCase();
  return ROOM_LEVELS.includes(normalized) ? normalized : "bronze";
}

export function normalizeRoomTheme(themeID) {
  return ROOM_THEMES[themeID] ? themeID : ROOM_DEFAULT_THEME_ID;
}

export function normalizeWindowScene(sceneID) {
  return ROOM_WINDOW_SCENES[sceneID] ? sceneID : ROOM_DEFAULT_WINDOW_SCENE_ID;
}

export function roomAssetPath(code, level) {
  return `/room-items/${code}/${normalizeRoomLevel(level)}.png`;
}
