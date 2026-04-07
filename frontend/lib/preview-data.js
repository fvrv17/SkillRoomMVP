export const REGIONS = [
  { id: "americas", label: "Americas", country: "US" },
  { id: "europe", label: "Europe", country: "DE" },
  { id: "mena", label: "MENA", country: "AE" },
  { id: "apac", label: "APAC", country: "SG" },
];

export const PREVIEW_TEMPLATES = [
  {
    id: "react_debug_resize_cleanup",
    title: "Stabilize the resize observer hook",
    category: "debug",
    difficulty: 3,
    description: "Fix listener cleanup so the room canvas stops leaking subscriptions during layout changes.",
    editableFiles: ["src/useRoomResize.js"],
    starterFiles: {
      "src/useRoomResize.js": `export function useRoomResize(onResize) {\n  window.addEventListener("resize", onResize);\n}\n`,
    },
    visibleTests: {
      "tests/useRoomResize.test.jsx": "cleans up the resize listener\\ninvokes the callback only once per event",
    },
  },
  {
    id: "react_feature_search",
    title: "Build candidate search filters",
    category: "feature",
    difficulty: 2,
    description: "Implement live search with debounced updates for recruiter candidate discovery.",
    editableFiles: ["src/App.jsx"],
    starterFiles: {
      "src/App.jsx": `export default function App() {\n  return <section>Search panel goes here</section>;\n}\n`,
    },
    visibleTests: {
      "tests/App.test.jsx": "filters by name\\nkeeps empty state readable",
    },
  },
  {
    id: "react_refactor_invite_form",
    title: "Refactor the interview invite form",
    category: "refactor",
    difficulty: 4,
    description: "Reduce state duplication while preserving validation and scheduling behavior.",
    editableFiles: ["src/InviteForm.jsx"],
    starterFiles: {
      "src/InviteForm.jsx": `export function InviteForm() {\n  return <form>Invite form</form>;\n}\n`,
    },
    visibleTests: {
      "tests/InviteForm.test.jsx": "submits valid invites\\nshows validation errors",
    },
  },
  {
    id: "react_logic_selection_state",
    title: "Repair the shortlist selection model",
    category: "logic",
    difficulty: 3,
    description: "Restore deterministic selection state so recruiter actions stay consistent across pagination.",
    editableFiles: ["src/useSelectionState.js"],
    starterFiles: {
      "src/useSelectionState.js": `export function useSelectionState() {\n  return { selected: [] };\n}\n`,
    },
    visibleTests: {
      "tests/useSelectionState.test.jsx": "keeps selected IDs across updates\\nremoves deselected IDs cleanly",
    },
  },
  {
    id: "react_performance_virtual_list",
    title: "Virtualize the results stream",
    category: "performance",
    difficulty: 5,
    description: "Render a large recruiter feed without collapsing scroll performance or selection state.",
    editableFiles: ["src/VirtualCandidateList.jsx"],
    starterFiles: {
      "src/VirtualCandidateList.jsx": `export function VirtualCandidateList() {\n  return <div>Virtual list</div>;\n}\n`,
    },
    visibleTests: {
      "tests/VirtualCandidateList.test.jsx": "renders the visible slice\\nkeeps scroll math stable",
    },
  },
];

export const PREVIEW_DATA = {
  user: {
    id: "preview-user",
    username: "Preview Operator",
    email: "preview@skillroom.dev",
    role: "user",
    country: "US",
  },
  profile: {
    user_id: "preview-user",
    selected_track: "react",
    current_skill_score: 618,
    percentile_global: 89,
    percentile_country: 92,
    confidence_score: 78,
    completed_challenges: 14,
    streak_days: 4,
  },
  skills: [
    { skill_code: "architecture", score: 540, confidence: 78, level: "silver" },
    { skill_code: "consistency", score: 610, confidence: 78, level: "gold" },
    { skill_code: "javascript", score: 575, confidence: 78, level: "silver" },
    { skill_code: "performance", score: 520, confidence: 78, level: "silver" },
    { skill_code: "react", score: 690, confidence: 78, level: "gold" },
  ],
  room: [
    {
      room_item_code: "monitor",
      current_level: "gold",
      state_json: {
        explanation: "React score drives a brighter multi-panel monitor rig.",
        linked_tasks: ["Search filters shipped", "Selection model debugged"],
      },
    },
    {
      room_item_code: "desk",
      current_level: "silver",
      state_json: {
        explanation: "JavaScript fundamentals reinforce the desk frame and storage.",
        linked_tasks: ["Search debounce", "Event cleanup"],
      },
    },
    {
      room_item_code: "chair",
      current_level: "silver",
      state_json: {
        explanation: "Architecture and performance work upgrade the strategy chair.",
        linked_tasks: ["Invite refactor", "Virtual list tuning"],
      },
    },
    {
      room_item_code: "plant",
      current_level: "gold",
      state_json: {
        explanation: "Consistency and streaks keep the plant healthy.",
        linked_tasks: ["4 day streak", "Low variance on last 5 scores"],
      },
    },
    {
      room_item_code: "trophy_case",
      current_level: "static",
      state_json: {
        presentation_mode: "achievement_case",
        case_variant: "default",
        achievement_count: 3,
        achievements: [
          {
            code: "challenge_volume_10",
            title: "10 verified challenges",
            description: "14 completed challenges create a reliable volume signal.",
          },
          {
            code: "category_coverage_4",
            title: "Cross-category range",
            description: "The preview profile shows validated work across multiple challenge categories.",
          },
          {
            code: "streak_3",
            title: "3-day streak",
            description: "A 4 day streak unlocked a consistency trophy.",
          },
        ],
        explanation: "3 verified trophies are currently on display.",
        linked_tasks: ["Global percentile 89", "Country percentile 92"],
      },
    },
    {
      room_item_code: "shelf",
      current_level: "silver",
      state_json: {
        explanation: "Solved task volume fills the library shelf.",
        linked_tasks: ["14 completed challenges", "5 categories covered"],
      },
    },
  ],
  rankings: [
    { rank: 1, username: "Ada Frame", country: "GB", current_skill_score: 812, confidence_score: 91, percentile: 99, completed_challenges: 24 },
    { rank: 2, username: "Lin Park", country: "SG", current_skill_score: 745, confidence_score: 88, percentile: 96, completed_challenges: 20 },
    { rank: 3, username: "Preview Operator", country: "US", current_skill_score: 618, confidence_score: 78, percentile: 89, completed_challenges: 14 },
    { rank: 4, username: "Sofia Ray", country: "AE", current_skill_score: 598, confidence_score: 80, percentile: 85, completed_challenges: 13 },
  ],
  candidates: [
    {
      user_id: "cand-1",
      username: "Mila Stone",
      country: "US",
      summary: {
        score: 702,
        percentile: 94,
        confidence_score: 84,
        confidence_level: "high",
        last_active_at: "2025-01-16T14:00:00Z",
        tasks_completed: 17,
      },
      current_skill_score: 702,
      percentile_global: 94,
      confidence_score: 84,
      confidence_level: "high",
      confidence_reasons: ["17 completed tasks make the score more reliable.", "No major anomaly signals were recorded."],
      tasks_solved: 17,
      recent_activity: ["Solved performance challenge 2h ago", "Hint-free debug pass yesterday"],
      strengths: ["React", "Performance"],
      weaknesses: ["Architecture"],
    },
    {
      user_id: "cand-2",
      username: "Ibrahim Noor",
      country: "AE",
      summary: {
        score: 641,
        percentile: 87,
        confidence_score: 79,
        confidence_level: "medium",
        last_active_at: "2025-01-16T10:00:00Z",
        tasks_completed: 15,
      },
      current_skill_score: 641,
      percentile_global: 87,
      confidence_score: 79,
      confidence_level: "medium",
      confidence_reasons: ["Multiple completed tasks make the score more stable.", "Recent challenge results have stayed consistent."],
      tasks_solved: 15,
      recent_activity: ["Submitted refactor challenge today", "Stable scores across 4 attempts"],
      strengths: ["JavaScript", "Consistency"],
      weaknesses: ["Performance"],
    },
  ],
};

export function createPreviewChallenge(templateID) {
  const template = PREVIEW_TEMPLATES.find((item) => item.id === templateID) || PREVIEW_TEMPLATES[0];
  return {
    instance: { id: `preview-${template.id}`, attempt_number: 1 },
    template_id: template.id,
    title: template.title,
    description_md: template.description,
    category: template.category,
    difficulty: template.difficulty,
    editable_files: [...template.editableFiles],
    visible_tests: { ...template.visibleTests },
    variant: {
      generated_files: { ...template.starterFiles },
      editable_files: [...template.editableFiles],
    },
  };
}
