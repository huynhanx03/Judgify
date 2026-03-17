/**
 * Vietnamese text constants for the UI.
 * All user-facing strings are defined here to support future i18n.
 * When adding multi-language support, replace this with a proper i18n library.
 */

export const TEXT = {
  // App-level
  APP_NAME: "Judgify",
  APP_DESCRIPTION: "Thánh Địa Luyện Code",
  APP_FULL_TITLE: "Judgify - Thánh Địa Luyện Code",

  // Navigation
  NAV: {
    ARENA: "Luyện Tập",
    MATERIALS: "Tàng Kinh Các",
    CONTEST: "Đại Hội Tỷ Thí",
    RANKING: "Bảng Phong Thần",
    PROFILE: "Hồ Sơ Tu Luyện",
    SETTINGS: "Cài Đặt",
    LOGOUT: "Đăng Xuất",
  },

  // Header User Dropdown
  HEADER: {
    AVATAR_FALLBACK: "ĐH",
    GUEST_NAME: "Đạo Hữu Vô Danh",
    GUEST_RANK: "Trúc Cơ Kỳ",
  },

  // Auth
  AUTH: {
    LOGIN: "Đăng Nhập",
    REGISTER: "Đăng Ký",
    USERNAME: "Pháp Danh",
    USERNAME_PLACEHOLDER: "Nhập pháp danh của bạn",
    PASSWORD: "Mật Khẩu",
    PASSWORD_PLACEHOLDER: "Nhập mật khẩu",
    FORGOT_PASSWORD: "Quên Mật Khẩu?",
    NO_ACCOUNT: "Chưa có tài khoản?",
    HAS_ACCOUNT: "Đã có tài khoản?",
    FIRST_NAME: "Họ",
    LAST_NAME: "Tên",
    BIRTHDAY: "Ngày Sinh",
    GENDER: "Giới Tính",
    GENDER_MALE: "Nam",
    GENDER_FEMALE: "Nữ",
    GENDER_OTHER: "Khác",
    WELCOME_BACK: "Chào Mừng Trở Lại",
    WELCOME_SUBTITLE: "Hãy đăng nhập để tiếp tục hành trình tu luyện",
  },

  // Arena
  ARENA: {
    TITLE: "Thánh Địa Luyện Tập",
    SUBTITLE: "Nơi ngưng tụ linh khí, rèn luyện pháp thuật và đột phá đạo cơ.",
    ALL: "Tất Cả",
    EASY: "Dễ",
    MEDIUM: "Trung",
    HARD: "Khó",
    PROBLEM_TITLE: "Tên Bài",
    DIFFICULTY: "Phẩm Chất",
    TAGS: "Công Pháp",
    ACCEPTANCE: "Tỉ Lệ AC",
    STATUS: "Trạng Thái",
    SOLVED: "Đã Giải",
    UNSOLVED: "Chưa Giải",
    ATTEMPTED: "Đang Thử",
    NO_PROBLEMS: "Chưa có bài tập nào",
    NO_PROBLEMS_DESC: "Đạo hữu có thể thử tìm kiếm hoặc điều chỉnh lại bộ lọc Phẩm Chất.",
    SEARCH_PLACEHOLDER: "Tìm kiếm danh tự bài tập...",
    // Filter Panel
    FILTER_BUTTON: "Lọc & Sắp Xếp",
    FILTER_TITLE: "Bộ Lọc & Sắp Xếp",
    SORT_TITLE: "Sắp Xếp Tiên Bảng",
    SORT_ACCEPTANCE: "Tỷ Lệ AC",
    SORT_SOLVED: "Đã Giải",
    DIFFICULTY_TITLE: "Phẩm Chất",
    TAGS_TITLE: "Công Pháp",
    CLEAR_FILTER: "Làm mới bộ lọc",
    ACTIVE_FILTERS: "Đang lọc:",
    CLEAR_ALL: "Xóa tất cả",
    ASC: "Tăng dần",
    DESC: "Giảm dần",
    DEFAULT: "Mặc định",
  },

  // Theme
  THEME: {
    LIGHT: "Sáng",
    DARK: "Tối",
    SYSTEM: "Hệ Thống",
    TOGGLE: "Chuyển đổi giao diện",
  },

  // Common
  COMMON: {
    LOADING: "Đang tải...",
    ERROR: "Có lỗi xảy ra",
    RETRY: "Thử lại",
    SAVE: "Lưu",
    CANCEL: "Hủy",
    DELETE: "Xóa",
    EDIT: "Sửa",
    CREATE: "Tạo Mới",
    SEARCH: "Tìm Kiếm",
    FILTER: "Lọc",
    NO_DATA: "Không có dữ liệu",
    PAGE: "Trang",
    OF: "trong số",
    PREVIOUS: "Trước",
    NEXT: "Tiếp",
  },
} as const;
