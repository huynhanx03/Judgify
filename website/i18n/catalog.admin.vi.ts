/**
 * Canonical Vietnamese administrator-surface copy.
 * Domain, user, contest, problem, material, and administrator-authored data
 * remains in API DTOs and must not be copied into this catalog.
 */

import { APP_LOCALE } from "@/i18n/locale";

/**
 * Capability metadata is owned by the API, but it is not user-facing copy.
 * Keep the presentation vocabulary here so a future locale can replace this
 * catalog without changing the permission matrix or server contract.
 */
const ADMIN_PERMISSION_RESOURCE_LABELS: Record<string, string> = {
  audit: "Nhật ký kiểm toán",
  authorization: "Phân quyền",
  attribute_definition: "Thuộc tính hồ sơ",
  contest: "Cuộc thi",
	cultivation_ledger: "Sổ cái tiến trình",
	cultivation_reward_rule: "Luật thưởng tu luyện",
  difficulty: "Độ khó",
  element: "Nguyên tố",
  level: "Cấp độ",
  judge: "Vận hành hệ thống chấm",
  material: "Tài liệu",
  material_category: "Danh mục tài liệu",
  notification: "Chiến dịch thông báo",
  operation: "Tác vụ nền",
  problem: "Bài tập",
  submission: "Bài nộp",
  rank: "Xếp hạng",
  rarity: "Độ hiếm",
  role: "Vai trò và quyền",
  tag: "Nhãn",
  test_case: "Bộ kiểm thử",
  trait: "Thiên phú",
  user: "Người dùng",
  user_trait: "Bản chiếu thiên phú người dùng",
};

const ADMIN_PERMISSION_RESOURCE_DESCRIPTIONS: Record<string, string> = {
  audit: "Tra cứu các sự kiện quản trị và bảo mật bất biến.",
  authorization: "Cấp vai trò và quản lý chính sách phân quyền.",
  attribute_definition: "Quản lý cấu trúc thuộc tính hồ sơ người dùng.",
  contest: "Tạo, lên lịch và vận hành các cuộc thi lập trình.",
	cultivation_ledger: "Tra cứu sổ cái tiến trình bất biến và tạo bút toán bù trừ có kiểm toán.",
	cultivation_reward_rule: "Tra cứu và công bố các revision công thức thưởng có thời điểm hiệu lực rõ ràng.",
  difficulty: "Quản lý danh mục độ khó của bài tập.",
  element: "Quản lý nguyên tố và cách hiển thị trong hệ thống.",
  level: "Quản lý các mốc tiến trình tu luyện.",
  judge: "Theo dõi worker, hàng đợi bền vững và runtime capability đã kích hoạt.",
  material: "Quản lý tài liệu học tập và nội dung biên tập.",
  material_category: "Sắp xếp tài liệu học tập theo danh mục.",
  notification:
    "Soạn, phê duyệt và theo dõi việc phát thông báo đến người dùng.",
  operation:
    "Theo dõi và điều khiển các tác vụ nền bền vững giữa nhiều miền nghiệp vụ.",
  problem: "Tạo và duy trì kho bài tập lập trình.",
  submission: "Theo dõi bài nộp và kiểm soát dữ liệu chấm nhạy cảm.",
  rank: "Quản lý cấp bậc và ngưỡng tiến trình.",
  rarity: "Quản lý các cấp độ hiếm của phần thưởng.",
  role: "Quản lý vai trò và chính sách phân quyền.",
  tag: "Quản lý hệ thống phân loại bài tập.",
  test_case: "Quản lý bộ kiểm thử công khai và riêng tư.",
  trait: "Quản lý thiên phú và hiệu ứng của chúng.",
  user: "Quản lý tài khoản và vai trò người dùng.",
  user_trait: "Chỉ đọc bản chiếu tương thích được dựng từ hồ sơ phần thưởng có hiệu lực.",
};

const ADMIN_PERMISSION_ACTION_LABELS: Record<string, string> = {
  create: "Tạo",
  read: "Xem",
  update: "Sửa",
  delete: "Xóa",
  inspect: "Xem mã nguồn",
  manage: "Quản lý",
  publish: "Xuất bản",
  archive: "Lưu trữ",
  rollback: "Khôi phục",
  execute: "Điều khiển",
	adjust: "Điều chỉnh",
	reverse: "Đảo bút toán",
	rebuild: "Đối soát lại",
  broadcast_all_active: "Gửi toàn hệ thống",
  broadcast_role: "Gửi theo vai trò",
  compose: "Soạn chiến dịch",
  read_delivery: "Xem kết quả phát",
};

const ADMIN_PERMISSION_ACTION_DESCRIPTIONS: Record<string, string> = {
  create: "Tạo bản ghi mới.",
  read: "Xem dữ liệu và cấu hình.",
  update: "Thay đổi bản ghi hiện có.",
  delete: "Xóa bản ghi hiện có.",
  inspect: "Xem mã nguồn và chẩn đoán chấm bài được giới hạn.",
  manage: "Cấp vai trò và thay đổi chính sách phân quyền nhạy cảm.",
  publish: "Công bố một revision đã được kiểm tra.",
  archive: "Đưa nội dung ra khỏi trạng thái đang phục vụ.",
  rollback: "Khôi phục dữ liệu về một revision trước đó.",
  execute: "Tạm dừng, tiếp tục hoặc hủy tác vụ nền.",
	adjust: "Tạo bút toán điều chỉnh có giới hạn và lý do rõ ràng.",
	reverse: "Tạo bút toán bù trừ chính xác cho một điều chỉnh trước đó.",
	rebuild: "Đối soát read-model từ nguồn dữ liệu bất biến.",
  broadcast_all_active:
    "Cho phép phát thông báo đến toàn bộ tài khoản đang hoạt động.",
  broadcast_role:
    "Cho phép phát thông báo đến thành viên của một vai trò.",
  compose: "Tạo, chỉnh sửa, duyệt và lên lịch chiến dịch thông báo.",
  read_delivery:
    "Xem trạng thái phát đến từng người nhận và mã lỗi đã được lược bỏ.",
};

function localizedPermissionFallback(key: string): string {
  return key
    .replace(/[_.:-]+/g, " ")
    .replace(/\b\w/g, (character) => character.toUpperCase());
}

function localizedPermissionResourceLabel(key: string): string {
  return ADMIN_PERMISSION_RESOURCE_LABELS[key] ?? localizedPermissionFallback(key);
}

function localizedPermissionResourceDescription(key: string): string | undefined {
  return ADMIN_PERMISSION_RESOURCE_DESCRIPTIONS[key];
}

function localizedPermissionActionLabel(key: string): string {
  return ADMIN_PERMISSION_ACTION_LABELS[key] ?? localizedPermissionFallback(key);
}

function localizedPermissionActionDescription(key: string): string | undefined {
  return ADMIN_PERMISSION_ACTION_DESCRIPTIONS[key];
}

/** Canonical bootstrap copy is localized only while it remains unedited. */
const ADMIN_CANONICAL_ROLE_NAMES: Record<
  string,
  { source: string; localized: string }
> = {
  admin: { source: "Admin", localized: "Quản trị viên" },
  student: { source: "Student", localized: "Học viên" },
  superadmin: {
    source: "Super Administrator",
    localized: "Quản trị viên tối cao",
  },
};

const ADMIN_CANONICAL_ROLE_DESCRIPTIONS: Record<
  string,
  { source: string; localized: string }
> = {
  superadmin: {
    source: "Protected break-glass administrator role",
    localized: "Vai trò quản trị khẩn cấp được hệ thống bảo vệ.",
  },
};

function localizedRoleName(key: string, value: string): string {
  const canonical = ADMIN_CANONICAL_ROLE_NAMES[key];
  return canonical?.source === value ? canonical.localized : value;
}

function localizedRoleDescription(
  key: string,
  value?: string,
): string | undefined {
  const canonical = ADMIN_CANONICAL_ROLE_DESCRIPTIONS[key];
  return canonical?.source === value ? canonical.localized : value;
}

export const admin = {
    RELATED_DATA_LOAD_ERROR:
      "Không thể tải dữ liệu liên quan. Một số bộ lọc hoặc trường nhập tạm thời chưa khả dụng.",
    // Dashboard
    DASHBOARD_TITLE: "Dashboard",
    DASHBOARD_SUBTITLE: "Tổng quan hệ thống Judgify",
    RECENT_ACTIVITY: "Hoạt Động Gần Đây",
    STAT_USERS: "Người Dùng",
    STAT_PROBLEMS: "Bài Tập",
    STAT_CONTESTS: "Cuộc Thi",
    STAT_TAGS: "Tags",
    STAT_ROLES: "Vai Trò",
    STAT_UNAVAILABLE: "Chưa thể tải",
    STATS_EMPTY: "Bạn chưa có quyền xem các chỉ số tổng quan.",
    STATS_LOAD_ERROR: "Một số chỉ số chưa tải được. Phiên đăng nhập vẫn được giữ.",
    PRODUCT_NAME: "Judgify Admin",
    BREADCRUMB_ROOT: "Admin",
    BREADCRUMB_LABEL: "Đường dẫn trang quản trị",
    NAVIGATION_LABEL: "Điều hướng quản trị",
    MOBILE_NAV_DESCRIPTION:
      "Chọn khu vực quản trị bạn muốn mở. Menu sẽ đóng sau khi điều hướng.",
    LOGOUT: "Đăng Xuất",
    COLUMN_ID: "ID",
    PROBLEM_NOT_FOUND: "Không tìm thấy bài tập",
    PROBLEM_LOADING: "Đang tải dữ liệu bài tập...",
    PROBLEM_LOAD_ERROR: "Không thể tải bài tập. Vui lòng thử lại.",
    ACTIVITY_EMPTY: "Chưa có hoạt động nào",
    ACTIVITY_EMPTY_DESCRIPTION:
      "Hoạt động quản trị sẽ xuất hiện khi nhật ký kiểm toán có dữ liệu.",
    ACCESS: {
      LOADING: "Đang xác minh quyền truy cập...",
      FORBIDDEN_CODE: "403",
      FORBIDDEN_TITLE: "Bạn không có quyền truy cập",
      FORBIDDEN_DESCRIPTION:
        "Tài khoản đã đăng nhập nhưng chưa được cấp quyền cho khu vực quản trị này.",
      UNAVAILABLE_LABEL: "Kết nối phân quyền bị gián đoạn",
      UNAVAILABLE_TITLE: "Chưa thể xác minh quyền truy cập",
      UNAVAILABLE_DESCRIPTION:
        "Phiên đăng nhập vẫn được giữ an toàn. Hãy thử lại khi dịch vụ sẵn sàng.",
      RETRY: "Thử xác minh lại",
      BACK_TO_PRODUCT: "Về trang luyện tập",
    },
    EXAMPLES: {
      DIFFICULTY: "VD: Dễ",
      ELEMENT: "VD: Hoả",
      LEVEL: "VD: Luyện Khí",
      RANK: "VD: Bạch Kim",
      RARITY_NAME: "VD: Thiên Phẩm",
      RARITY_CODE: "VD: LEGENDARY",
      TAG: "VD: Quy Hoạch Động",
      TRAIT: "VD: Kim Cương Căn",
    },
    OPTIONAL: "Tuỳ chọn",
    SELECT_CATEGORY: "— Chọn danh mục —",
    SELECT_DIFFICULTY: "— Chọn độ khó —",
    ELEMENT_FORM: {
      REQUIRED: "Tên và Code không được để trống",
      UPDATE_SUCCESS: "Cập nhật thành công",
      CREATE_SUCCESS: "Tạo thành công",
      NAME: "Tên",
      NAME_PLACEHOLDER: "Nhập tên nguyên tố",
      CODE: "Code",
      CODE_PLACEHOLDER: "Ví dụ: FIRE, WATER",
      DESCRIPTION: "Mô Tả",
      DESCRIPTION_PLACEHOLDER: "Mô tả ngắn (tuỳ chọn)",
      UPDATE_ERROR: "Không thể cập nhật",
      CREATE_ERROR: "Không thể tạo",
    },
    PERMISSIONS: {
      RESOURCE: "Tài Nguyên",
      ACTIONS: "Hành Động Được Phép",
      CATALOG_REVISION: (revision: number) => `Phiên bản danh mục: ${revision}`,
      EMPTY_TITLE: "Danh mục quyền đang trống",
      EMPTY_DESCRIPTION:
        "Hệ thống chưa công bố tài nguyên và hành động nào để phân quyền.",
      NO_ACTIONS: "Tài nguyên này chưa có hành động.",
      GRANT: (action: string, resource: string) =>
        `Cấp quyền ${action} trên ${resource}`,
      REVOKE: (action: string, resource: string) =>
        `Thu hồi quyền ${action} trên ${resource}`,
      RESOURCE_LABEL: localizedPermissionResourceLabel,
      RESOURCE_DESCRIPTION: localizedPermissionResourceDescription,
      ACTION_LABEL: localizedPermissionActionLabel,
      ACTION_DESCRIPTION: localizedPermissionActionDescription,
      CREATE: "Tạo",
      CREATE_TITLE: "Create",
      READ: "Xem",
      READ_TITLE: "Read",
      UPDATE: "Sửa",
      UPDATE_TITLE: "Update",
      DELETE: "Xóa",
      DELETE_TITLE: "Delete",
    },
    PROBLEM_FORM: {
      TEST_CASE_REQUIRED: "Input và Expected Output không được để trống",
      CONTENT_REQUIRED: (field: string) => `${field} không được để trống.`,
      CONTENT_CONTAINS_NUL: (field: string) =>
        `${field} không được chứa byte NUL.`,
      CONTENT_TOO_LARGE: (field: string, maximum: string) =>
        `${field} vượt quá giới hạn ${maximum}.`,
      CONTENT_SIZE: (actual: string, maximum: string) =>
        `${actual} / ${maximum}`,
      TEST_CASE_UPDATED: "Test case đã cập nhật",
      TEST_CASE_CREATED: "Test case đã tạo",
      TEST_CASE_DELETED: "Test case đã xóa",
      UPDATE_SUCCESS: "Cập nhật thành công",
      CREATE_SUCCESS: "Tạo thành công",
      TITLE: "Tiêu đề",
      TITLE_PLACEHOLDER: "VD: Two Sum",
      TITLE_REQUIRED: "Tiêu đề không được để trống.",
      TITLE_TOO_LONG: (maximum: number) =>
        `Tiêu đề không được vượt quá ${maximum} ký tự.`,
      DIFFICULTY: "Độ khó",
      SELECT: "Chọn",
      DIFFICULTIES_LOADING: "Đang tải danh sách độ khó...",
      DIFFICULTIES_LOAD_ERROR_TITLE: "Không thể tải danh sách độ khó",
      DIFFICULTIES_LOAD_ERROR_DESCRIPTION:
        "Hãy thử lại trước khi chọn độ khó khác. Giá trị hiện tại vẫn được giữ an toàn.",
      DIFFICULTIES_EMPTY:
        "Chưa có độ khó nào. Hãy tạo ít nhất một độ khó trước khi tạo bài tập.",
      DIFFICULTIES_RESTRICTED:
        "Bạn không có quyền xem danh mục độ khó. Giá trị hiện tại chỉ được giữ nguyên.",
      TIME_LIMIT: "Time Limit (ms)",
      TIME_LIMIT_INVALID: (minimum: number, maximum: number) =>
        `Time Limit phải là số nguyên từ ${minimum} đến ${maximum} ms.`,
      MEMORY_LIMIT: "Memory Limit (KB)",
      MEMORY_LIMIT_INVALID: (minimum: number, maximum: number) =>
        `Memory Limit phải là số nguyên từ ${minimum} đến ${maximum} KB.`,
      STATUS: "Trạng thái",
      TAGS: "Tags",
      TAGS_LOADING: "Đang tải danh sách tags...",
      TAGS_LOAD_ERROR_TITLE: "Không thể tải danh sách tags",
      TAGS_LOAD_ERROR_DESCRIPTION:
        "Bạn vẫn có thể lưu bài tập; các tag hiện có được giữ nguyên cho đến khi tải lại thành công.",
      TAGS_EMPTY: "Chưa có tag nào để gắn cho bài tập.",
      TAGS_RESTRICTED:
        "Bạn không có quyền xem danh mục tags. Các tag hiện có chỉ được hiển thị để tham chiếu.",
      DESCRIPTION: "Mô tả (Markdown)",
      DESCRIPTION_REQUIRED: "Mô tả bài tập không được để trống.",
      EDITOR_LOADING: "Đang tải trình soạn thảo...",
      TEST_CASES: "Test Cases",
      TEST_CASES_DESCRIPTION: "Thêm, sửa hoặc xóa test case cho bài tập.",
      TEST_CASES_LOADING: "Đang tải test cases...",
      TEST_CASES_LOAD_ERROR_TITLE: "Không thể tải test cases",
      TEST_CASES_LOAD_ERROR_DESCRIPTION:
        "Không thể xác nhận dữ liệu kiểm thử hiện tại. Hãy tải lại trước khi thêm hoặc chỉnh sửa.",
      TEST_CASES_RESTRICTED:
        "Bạn không có quyền xem test case hiện có. Các thao tác được hiển thị theo quyền riêng đã cấp.",
      ADD: "Thêm",
      TEST_CASES_EMPTY: "Chưa có test case.",
      TEST_CASE_NUMBER: (index: number) => `Test Case #${index}`,
      INPUT: "Input",
      INPUT_PLACEHOLDER: "Dữ liệu đầu vào...",
      EXPECTED_OUTPUT: "Expected Output",
      EXPECTED_OUTPUT_PLACEHOLDER: "Kết quả mong đợi...",
      CREATE_BEFORE_TEST_CASES:
        "Tạo bài tập trước, sau đó thêm test case ở trang chỉnh sửa.",
      DELETE_TEST_CASE_ERROR: "Xóa test case thất bại",
      DELETE_TEST_CASE_TITLE: "Xóa test case?",
      DELETE_TEST_CASE_DESCRIPTION:
        "Test case này sẽ bị xóa vĩnh viễn và có thể làm thay đổi kết quả chấm bài.",
      DELETE_TEST_CASE_CONFIRM: "Xóa test case",
      SAVE_TEST_CASE_ERROR: "Lưu test case thất bại",
      EDIT_TITLE: "Chỉnh Sửa Bài Tập",
      CREATE_TITLE: "Tạo Bài Tập Mới",
      EDIT_DESCRIPTION: "Cập nhật thông tin bài tập.",
      CREATE_DESCRIPTION: "Thêm bài tập mới vào hệ thống.",
      PUBLISHED: "Published",
      DRAFT: "Draft",
      HIDDEN_TITLE: "Ẩn (hidden)",
      VISIBLE_TITLE: "Hiện (visible)",
      BACK_LABEL: "Quay lại danh sách bài tập",
      PUBLISH_TOGGLE_LABEL: "Thay đổi trạng thái xuất bản",
      REMOVE_TEST_CASE_LABEL: (index: number) => `Xóa test case số ${index}`,
      VISIBILITY_TOGGLE_LABEL: (index: number) =>
        `Thay đổi chế độ hiển thị của test case số ${index}`,
      HIDDEN: "Hidden",
      VISIBLE: "Visible",
      UPDATE: "Cập nhật",
      SAVE: "Lưu",
      SAVE_CHANGES: "Lưu thay đổi",
      CREATE_PROBLEM: "Tạo bài tập",
      FORM_HAS_ERRORS:
        "Hãy kiểm tra các trường bắt buộc và giới hạn trước khi lưu.",
      IMMUTABLE_DESCRIPTION:
        "Soạn nội dung, chính sách chấm và toàn bộ testset trong một bản nháp có phiên bản. Bản đã xuất bản luôn bất biến cho đến lần xuất bản kế tiếp.",
      RELEASE: (releaseID: string) => `Release chấm: ${releaseID}`,
      AUTHORING_RESTRICTED:
        "Tài khoản cần quyền tạo và cập nhật bài tập để hoàn tất toàn bộ quy trình authoring.",
      CATALOG_LOADING: "Đang tải chính sách chấm của release hiện tại...",
      CATALOG_ERROR_TITLE: "Không thể tải chính sách authoring",
      CATALOG_ERROR_DESCRIPTION:
        "Không thể lưu an toàn khi chưa xác minh runtime, checker và giới hạn từ máy chủ. Hãy thử tải lại.",
      CATALOG_REQUIRED:
        "Chính sách authoring chưa sẵn sàng. Hãy tải lại catalog trước khi lưu.",
      CATALOG_STALE_TITLE: "Catalog authoring đã thay đổi",
      CATALOG_STALE_DESCRIPTION:
        "Catalog đang dùng đã cũ. Nội dung đang soạn vẫn được giữ nguyên; hãy tải catalog mới nhất và kiểm tra các lựa chọn không còn tương thích trước khi lưu lại.",
      CATALOG_STALE_ERROR:
        "Catalog đã thay đổi nên thao tác chưa được gửi. Hãy kiểm tra lại runtime, checker và giới hạn.",
      CATALOG_VERIFY_ERROR:
        "Không thể xác minh catalog mới nhất nên thao tác chưa được gửi. Hãy thử lại khi kết nối ổn định.",
      RELOAD_CATALOG: "Tải lại catalog",
      VALIDATION_TITLE: "Bản nháp chưa sẵn sàng",
      CONFLICT_TITLE: "Có phiên bản mới hơn trên máy chủ",
      CONFLICT_DESCRIPTION:
        "Bản đang mở đã cũ. Tải dữ liệu mới nhất rồi áp dụng lại thay đổi để tránh ghi đè công việc của người khác.",
      CONFLICT_ERROR:
        "Bài tập vừa được cập nhật ở nơi khác. Hãy tải phiên bản mới nhất.",
      RELOAD_LATEST: "Tải bản mới nhất",
      RELOADING_LATEST: "Đang tải bản mới nhất...",
      RELOAD_LATEST_SUCCESS:
        "Đã tải metadata bản mới nhất và giữ nguyên nội dung đang soạn để bạn áp dụng lại.",
      RELOAD_LATEST_ERROR:
        "Không thể tải bản mới nhất. Nội dung đang soạn vẫn được giữ nguyên.",
      CONTENT_TITLE: "Nội dung bài tập",
      CONTENT_DESCRIPTION:
        "Tiêu đề, định danh ổn định và đề bài Markdown có hỗ trợ công thức toán.",
      SLUG: "Slug",
      SLUG_PLACEHOLDER: "two-sum",
      SLUG_HINT:
        "Dùng chữ thường, số và dấu gạch ngang; slug trở thành định danh URL ổn định.",
      SLUG_IMMUTABLE: "Slug không đổi sau khi aggregate được tạo.",
      SLUG_INVALID: (maximum: number) =>
        `Slug phải ở dạng kebab-case và không vượt quá ${maximum} byte.`,
      TITLE_INVALID: (maximum: number) =>
        `Tiêu đề là bắt buộc và không vượt quá ${maximum} byte.`,
      STATEMENT: "Đề bài (Markdown)",
      STATEMENT_INVALID: (maximum: number) =>
        `Đề bài là bắt buộc và không vượt quá ${maximum} byte.`,
      DIFFICULTY_REQUIRED: "Hãy chọn một độ khó hợp lệ.",
      DIFFICULTY_UNAVAILABLE: (id: string) =>
        `Độ khó ${id} không còn trong danh mục hiện tại; hãy chọn giá trị mới.`,
      TAGS_LIMIT: (maximum: number) =>
        `Một bài tập được gắn tối đa ${maximum} tag.`,
      LIMITS_INVALID:
        "Giới hạn tài nguyên nằm ngoài chính sách release hoặc wall time nhỏ hơn CPU time.",
      TESTSET_TITLE: "Testset bất biến",
      TESTSET_DESCRIPTION:
        "Nhóm test quyết định thứ tự và chiến lược dừng. Case ẩn chỉ xuất hiện trong editor được phân quyền và worker chấm bài.",
      GROUP_COUNT: (count: number) => `${count} nhóm`,
      CASE_COUNT: (count: number) => `${count} test`,
      ADD_GROUP: "Thêm nhóm",
      NO_GROUP_TITLE: "Chưa có nhóm test",
      NO_GROUP_DESCRIPTION:
        "Thêm ít nhất một nhóm và một test ẩn trước khi lưu.",
      GROUP_NUMBER: (index: number) => `Nhóm ${index}`,
      GROUP_CASES: (count: number) => `${count} test trong nhóm`,
      STOP_ON_FAILURE: "Dừng nhóm khi sai",
      ADD_CASE: "Thêm test",
      REMOVE_GROUP: (index: number) => `Xóa nhóm test ${index}`,
      CASE_KIND: (group: number, index: number) =>
        `Loại test ${index} trong nhóm ${group}`,
      SAMPLE_CASE: "Ví dụ công khai",
      HIDDEN_CASE: "Test chấm ẩn",
      REMOVE_CASE: (group: number, index: number) =>
        `Xóa test ${index} trong nhóm ${group}`,
      TESTSET_LIMIT: (groups: number, cases: number) =>
        `Testset cần nhóm không rỗng, tối đa ${groups} nhóm và ${cases} test.`,
      HIDDEN_TEST_REQUIRED:
        "Cần ít nhất một test ẩn để bài tập có thể được chấm.",
      TEST_CONTENT_INVALID: (maximum: number) =>
        `Input và output của mỗi test có thể để trống, nhưng không chứa NUL và không vượt quá ${maximum} byte.`,
      JUDGE_POLICY_TITLE: "Giới hạn thực thi",
      JUDGE_POLICY_DESCRIPTION:
        "Các giá trị này được đóng băng trong revision và áp dụng cho từng test.",
      SELECT_DIFFICULTY: "— Chọn độ khó —",
      CPU_LIMIT: "CPU (ms)",
      WALL_LIMIT: "Wall (ms)",
      OUTPUT_LIMIT: "Output (byte)",
      PROCESS_LIMIT: "Số tiến trình",
      RUNTIME_CHECKER_TITLE: "Runtime & checker",
      RUNTIME_CHECKER_DESCRIPTION:
        "Chỉ các profile đã được phê duyệt trong release hiện tại mới có thể xuất bản.",
      ALLOWED_RUNTIMES: "Runtime được phép",
      RUNTIME_VERSION: (version: string) => `Profile ${version}`,
      RUNTIME_INCOMPATIBLE:
        "Runtime này không hỗ trợ checker đang chọn.",
      RUNTIME_UNAVAILABLE: (key: string) =>
        `${key} không còn trong release — bấm để loại bỏ`,
      RUNTIME_REQUIRED:
        "Chọn ít nhất một runtime đang hoạt động trong release hiện tại.",
      RUNTIME_LIMIT: (maximum: number) =>
        `Chỉ được chọn tối đa ${maximum} runtime trong release hiện tại.`,
      CHECKER: "Checker",
      SELECT_CHECKER: "— Chọn checker —",
      CHECKER_OPTION: (key: string, version: string) => `${key} · ${version}`,
      CHECKER_UNAVAILABLE: (key: string, version: string) =>
        `Checker ${key} · ${version} không còn trong release hiện tại.`,
      CHECKER_REQUIRED:
        "Chọn một checker đang hoạt động trong release hiện tại.",
      CAPABILITY_PAIR_UNAVAILABLE:
        "Checker đã chọn không hỗ trợ tất cả runtime đang bật. Hãy chọn checker tương thích hoặc bỏ runtime không phù hợp.",
      CHECKER_INCOMPATIBLE: (key: string, version: string) =>
        `${key} · ${version} — không hỗ trợ bộ runtime đang chọn`,
      CHECKER_COMPATIBILITY_HINT:
        "Checker khả dụng được lọc theo toàn bộ runtime đang chọn.",
      ABSOLUTE_TOLERANCE: "Sai số tuyệt đối",
      RELATIVE_TOLERANCE: "Sai số tương đối",
      TOLERANCE_PLACEHOLDER: "0…1",
      TOLERANCE_INVALID:
        "Sai số tuyệt đối và tương đối phải là số từ 0 đến 1.",
      TAXONOMY_TITLE: "Phân loại & phần thưởng",
      TAXONOMY_DESCRIPTION:
        "Độ khó và tag được snapshot để kết quả thưởng không thay đổi theo dữ liệu tương lai.",
      AUDIT_REASON: "Lý do thay đổi",
      AUDIT_REASON_DESCRIPTION:
        "Bắt buộc cho mỗi lần lưu, xuất bản hoặc lưu trữ để truy vết bằng CID.",
      AUDIT_REASON_PLACEHOLDER:
        "Mô tả ngắn gọn mục đích của thay đổi này...",
      REASON_INVALID: (maximum: number) =>
        `Lý do là bắt buộc và không vượt quá ${maximum} byte.`,
      DRAFT_SAVED: "Đã lưu một revision bản nháp mới.",
      SAVE_DRAFT_ERROR: "Không thể lưu bản nháp.",
      INITIAL_DRAFT_ROLLBACK_REASON:
        "Tự động lưu trữ shell tạo mới vì không thể lưu revision bản nháp đầu tiên.",
      PUBLISH_SUCCESS: "Đã xuất bản đúng revision và testset vừa chọn.",
      PUBLISH_ERROR: "Không thể xuất bản bài tập.",
      RENDER_REJECTED:
        "Bản render bị từ chối; hãy sửa chẩn đoán trước khi xuất bản.",
      RENDER_DIAGNOSTIC: (code: string) => `Render: ${code}`,
      ARCHIVE_SUCCESS: "Đã lưu trữ bài tập.",
      ARCHIVE_ERROR: "Không thể lưu trữ bài tập.",
      SAVING: "Đang lưu bản nháp...",
      PUBLISHING: "Đang xuất bản...",
      ARCHIVING: "Đang lưu trữ...",
      UNSAVED_CHANGES: "Có thay đổi chưa lưu",
      ALL_CHANGES_SAVED: "Nội dung đã khớp revision gần nhất",
      VERSION: (version: number) => `Optimistic version ${version}`,
      NOT_CREATED: "Aggregate chưa được tạo",
      ARCHIVE: "Lưu trữ",
      SAVE_DRAFT: "Lưu bản nháp",
      PUBLISH: "Xuất bản",
      LEAVE_TITLE: "Rời editor và bỏ thay đổi?",
      LEAVE_DESCRIPTION:
        "Những nội dung chưa tạo revision sẽ không được giữ lại.",
      LEAVE_CONFIRM: "Rời editor",
      ARCHIVE_TITLE: "Lưu trữ bài tập?",
      ARCHIVE_DESCRIPTION:
        "Bài tập sẽ biến mất khỏi danh sách công khai, nhưng revision đã xuất bản và lịch sử audit vẫn được giữ nguyên.",
      ARCHIVE_CONFIRM: "Xác nhận lưu trữ",
      ARCHIVED: "Đã lưu trữ",
      ARCHIVED_READ_ONLY:
        "Bài tập đã lưu trữ là dữ liệu chỉ đọc. Revision và lịch sử audit được giữ nguyên; không thể lưu hoặc xuất bản lại.",
      ARCHIVE_DIRTY_HINT:
        "Hãy lưu bản nháp trước khi lưu trữ để không mất thay đổi cục bộ.",
      TAG_UNAVAILABLE: (id: string) =>
        `Tag ${id} không còn trong danh mục — bấm để loại bỏ`,
    },

    NAV: {
      OVERVIEW: "Tổng Quan",
      CONTENT: "Quản Lý Nội Dung",
      OPERATIONS: "Vận Hành",
      CULTIVATION: "Tu Luyện",
      SYSTEM: "Quản Lý Hệ Thống",
    },

	PROGRESSION: {
	  EYEBROW: "Kiểm soát tiến trình",
	  TITLE: "Sổ Cái Tu Luyện",
	  SUBTITLE:
		"Mọi thay đổi EXP đều được ghi thành bút toán bất biến. Điều chỉnh thủ công chỉ được thực hiện qua preview, khóa phiên bản, lý do và mã sự cố để có thể truy vết đầy đủ.",
	  TABS_LABEL: "Khu vực quản lý tiến trình",
	  LEDGER_TAB: "Sổ cái & điều chỉnh",
	  RULE_TAB: "Revision luật thưởng",
	  LEDGER_QUERY_TITLE: "Tra cứu tiến trình người dùng",
	  LEDGER_QUERY_DESC:
		"Nhập UUID chính xác để xem lịch sử nguồn thưởng, số dư trước/sau, phiên bản và CID.",
	  USER_ID: "UUID người dùng",
	  USER_ID_PLACEHOLDER: "00000000-0000-0000-0000-000000000000",
	  SEARCH: "Tra cứu",
	  LEDGER_LOAD_ERROR: "Không thể tải sổ cái tiến trình.",
	  LEDGER_EMPTY: "Người dùng này chưa có bút toán tiến trình.",
	  HISTORY_TITLE: "Lịch sử bút toán bất biến",
	  HISTORY_DESC: "Dữ liệu chỉ đọc, sắp xếp mới nhất trước và phân trang bằng cursor ổn định.",
	  CORRELATION_ID: "Mã tương quan",
	  RECONCILIATION_TITLE: "Đối soát bảng xếp hạng",
	  RECONCILIATION_DESC:
		"Quét theo batch có checkpoint từ dữ liệu tiến trình gốc để phát hiện và sửa projection bị bỏ sót. Public ID và lựa chọn riêng tư của người dùng được giữ nguyên.",
	  RECONCILIATION_REVISION: "Revision projection",
	  LAST_RECONCILED: "Đã đối soát đến",
	  NEVER_RECONCILED: "Chưa đối soát",
	  LATEST_RECONCILIATION: "Tác vụ gần nhất",
	  NO_RECONCILIATION: "Chưa có tác vụ",
	  RECONCILIATION_REASON: "Lý do đối soát",
	  RECONCILIATION_REASON_PLACEHOLDER: "Mô tả sự cố hoặc căn cứ cần đối soát projection...",
	  START_RECONCILIATION: "Bắt đầu đối soát",
	  CONFIRM_RECONCILIATION: "Xác nhận và bắt đầu",
	  RECONCILIATION_SOURCE_COUNT: "Bản ghi trong phạm vi",
	  RECONCILIATION_REPAIR_COUNT: "Projection cần sửa",
	  RECONCILIATION_PREVIEW_REVISION: "Revision đã kiểm tra",
	  RECONCILIATION_PREVIEW_CUTOFF: "Mốc dữ liệu khóa",
	  RECONCILIATION_REFRESH: "Làm mới trạng thái đối soát",
	  RECONCILIATION_STATUS_ERROR: "Không thể tải trạng thái đối soát bảng xếp hạng.",
	  RECONCILIATION_PREVIEW_ERROR: "Không thể tạo bản xem trước đối soát. Vui lòng thử lại.",
	  RECONCILIATION_START_ERROR: "Không thể bắt đầu đối soát. Bản xem trước có thể đã cũ hoặc một tác vụ khác đang hoạt động; hãy xem trước lại.",
	  RECONCILIATION_STARTED: "Đã lên lịch đối soát bảng xếp hạng.",
	  RECONCILIATION_REPLAYED: "Đã khôi phục đúng tác vụ đối soát trước đó.",
	  ADJUSTMENT_TITLE: "Tạo điều chỉnh thủ công",
	  ADJUSTMENT_DESC:
		"Preview số dư trước khi xác nhận. Hệ thống sẽ từ chối nếu phiên bản đã thay đổi.",
	  REVERSAL_TITLE: "Đảo bút toán điều chỉnh",
	  REVERSAL_DESC:
		"Tạo bút toán bù trừ chính xác; bút toán gốc vẫn được giữ nguyên để kiểm toán.",
	  REVERSING_ENTRY: "Bút toán gốc:",
	  DELTA: "Chênh lệch EXP",
	  INCIDENT_ID: "Mã sự cố / phiếu hỗ trợ",
	  INCIDENT_PLACEHOLDER: "VD: INC-2026-0042",
	  REASON: "Lý do",
	  REASON_PLACEHOLDER: "Mô tả nguyên nhân và căn cứ của điều chỉnh...",
	  BEFORE: "Trước",
	  AFTER: "Sau",
	  PREVIEW: "Xem trước",
	  CONFIRM_ADJUSTMENT: "Ghi bút toán",
	  CONFIRM_REVERSAL: "Xác nhận bù trừ",
	  REVERSE: "Đảo bút toán",
	  PREVIEW_ERROR: "Không thể tạo bản xem trước. Hãy kiểm tra dữ liệu và tải lại sổ cái.",
	  APPLY_ERROR: "Không thể ghi bút toán. Dữ liệu có thể vừa thay đổi; hãy preview lại.",
	  ADJUSTMENT_SUCCESS: "Đã ghi bút toán điều chỉnh.",
	  REVERSAL_SUCCESS: "Đã ghi bút toán bù trừ.",
	  LOAD_MORE: "Tải thêm",
	  RULE_PUBLISH_TITLE: "Công bố revision luật thưởng",
	  RULE_PUBLISH_DESC:
		"Revision mới có hiệu lực theo đồng hồ cơ sở dữ liệu; lịch sử cũ không bao giờ bị diễn giải lại.",
	  MULTIPLIER: "Hệ số (basis points)",
	  FLAT_BONUS: "EXP cộng cố định",
	  RULE_REASON_PLACEHOLDER: "Mô tả lý do thay đổi công thức thưởng...",
	  RULE_PREVIEW_SUMMARY: (current: number, next: number) =>
		`Sẵn sàng công bố revision ${next}, kế tiếp revision ${current}.`,
	  PUBLISH_RULE: "Công bố revision",
	  RULE_HISTORY_TITLE: "Lịch sử revision",
	  RULE_HISTORY_DESC: "Mỗi revision là một snapshot bất biến với checksum và thời điểm hiệu lực.",
	  RULE_LOAD_ERROR: "Không thể tải lịch sử luật thưởng.",
	  RULE_EMPTY: "Chưa có revision luật thưởng nào.",
	  RETRY: "Thử tải lại",
	  RULE_PREVIEW_ERROR: "Không thể tạo bản xem trước luật thưởng.",
	  RULE_PUBLISH_ERROR: "Không thể công bố revision. Hãy tải lại và preview lại.",
	  RULE_PUBLISH_SUCCESS: "Đã công bố revision luật thưởng.",
	  RULE_FORMULA: (multiplier: number, bonus: number) =>
		`Hệ số ${multiplier} bp · cộng cố định ${bonus} EXP`,
	  REVISION: (revision: number) => `Revision ${revision}`,
	  LOADING: "Đang tải dữ liệu...",
	},

    SUBMISSIONS: {
      TITLE: "Giám Sát Bài Nộp",
      SUBTITLE:
        "Theo dõi hàng chấm, kết quả và tài nguyên thực thi từ dữ liệu hệ thống.",
      REFRESH: "Làm mới",
      REFRESHING: "Đang làm mới...",
      UPDATED_AT: (value: string) => `Cập nhật lúc ${value}`,
      FILTERS_TITLE: "Bộ lọc vận hành",
      FILTERS_DESCRIPTION:
        "Lọc chính xác theo trạng thái, runtime hoặc định danh hệ thống.",
      STATUS: "Trạng thái",
      ALL_STATUSES: "Tất cả trạng thái",
      LANGUAGE: "Runtime",
      ALL_LANGUAGES: "Tất cả runtime",
      RUNTIME_LOADING: "Đang tải runtime...",
      RUNTIME_UNAVAILABLE: "Không thể tải danh mục runtime lúc này.",
      PROBLEM_ID: "Problem ID",
      USER_ID: "User ID",
      CONTEST_ID: "Contest ID",
      UUID_PLACEHOLDER: "Nhập UUID chính xác",
      UUID_INVALID: "Định danh phải là UUID hợp lệ.",
      APPLY_FILTERS: "Áp dụng bộ lọc",
      RESET_FILTERS: "Xóa bộ lọc",
      LOAD_ERROR_TITLE: "Không thể tải danh sách bài nộp",
      LOAD_ERROR_DESCRIPTION:
        "Dữ liệu vận hành tạm thời không khả dụng. Hãy thử lại mà không cần tải lại trang.",
      EMPTY_TITLE: "Không tìm thấy bài nộp",
      EMPTY_DESCRIPTION:
        "Thử thay đổi bộ lọc hoặc làm mới để lấy trạng thái mới nhất.",
      CURRENT_PAGE: "Tóm tắt trang hiện tại",
      QUEUED: "Đang chờ",
      IN_PROGRESS: "Đang chấm",
      FINISHED: "Đã hoàn tất",
      TOTAL_ON_PAGE: "Tổng trên trang",
      TABLE_LABEL: "Danh sách bài nộp đang giám sát",
      COLUMN_SUBMISSION: "Bài nộp",
      COLUMN_STATUS: "Trạng thái",
      COLUMN_RUNTIME: "Runtime",
      COLUMN_PROGRESS: "Tiến độ",
      COLUMN_RESOURCES: "Tài nguyên",
      COLUMN_CONTEXT: "Ngữ cảnh",
      CREATED_AT: "Nộp lúc",
      UPDATED: "Cập nhật",
      PROBLEM: "Problem",
      USER: "User",
      CONTEST: "Contest",
      NO_CONTEST: "Ngoài cuộc thi",
      TESTS: "test",
      INSPECT: "Xem source",
      INSPECT_ARIA: (id: string) => `Xem source và chẩn đoán của bài nộp ${id}`,
      INSPECT_RESTRICTED:
        "Bạn có quyền xem danh sách nhưng chưa được cấp quyền đọc source và chẩn đoán.",
      DETAIL_TITLE: "Chi Tiết Bài Nộp",
      DETAIL_DESCRIPTION:
        "Source code và chẩn đoán được bảo vệ bởi quyền kiểm tra riêng.",
      DETAIL_LOADING: "Đang tải source và chẩn đoán...",
      DETAIL_ERROR_TITLE: "Không thể tải chi tiết bài nộp",
      DETAIL_ERROR_DESCRIPTION:
        "Quyền truy cập hoặc dữ liệu có thể vừa thay đổi. Hãy thử lại.",
      RETRY_DETAIL: "Thử tải lại chi tiết",
      VERSION: (version: number) => `Verdict v${version}`,
      NEVER_JUDGED: "Chưa hoàn tất chấm",
      OPERATIONS_TITLE: "Thao tác vận hành",
      OPERATIONS_DESCRIPTION:
        "Các thao tác được ràng buộc với phiên bản và thế hệ đang hiển thị để tránh thay đổi dữ liệu mới hơn.",
      UPDATE_RESTRICTED:
        "Bạn có thể xem chi tiết nhưng chưa được cấp quyền vận hành lại hoặc hủy bài chấm.",
      REJUDGE: "Chấm lại",
      CANCEL_JUDGING: "Hủy lần chấm này",
      REJUDGE_TITLE: "Chấm lại bài nộp?",
      REJUDGE_DESCRIPTION:
        "Máy chủ sẽ tạo một thế hệ chấm mới từ cấu hình bất biến đã lưu. Kết quả đang hoạt động không bị thay đổi cho đến khi thế hệ mới hoàn tất.",
      CANCEL_TITLE: "Hủy lần chấm đang chạy?",
      CANCEL_DESCRIPTION:
        "Chỉ thế hệ đang hiển thị sẽ bị hủy. Một lần chấm lại hoặc cập nhật mới hơn sẽ không bị ảnh hưởng.",
      REASON_LABEL: "Lý do bắt buộc",
      REASON_PLACEHOLDER: "Nêu ngắn gọn lý do vận hành...",
      REASON_HELP: (minimum: number, maximum: number) =>
        `Từ ${minimum} đến ${maximum} ký tự; không dùng ký tự điều khiển.`,
      REASON_INVALID: "Vui lòng nhập lý do hợp lệ trong giới hạn cho phép.",
      REJUDGE_CONFIRM: "Xác nhận chấm lại",
      CANCEL_CONFIRM: "Xác nhận hủy chấm",
      OPERATION_PENDING: "Đang gửi thao tác...",
      REJUDGE_SUCCESS: "Đã tạo thế hệ chấm lại mới.",
      CANCEL_SUCCESS: "Đã yêu cầu hủy thế hệ chấm đang chạy.",
      OPERATION_CONFLICT:
        "Trạng thái bài nộp vừa thay đổi. Dữ liệu mới nhất đã được tải lại; hãy kiểm tra trước khi thử lại.",
      OPERATION_ERROR: "Không thể hoàn tất thao tác vận hành. Vui lòng thử lại.",
    },

    JUDGE: {
      TITLE: "Trung Tâm Vận Hành Judge",
      SUBTITLE:
        "Quan sát hàng đợi bền vững, worker và capability runtime đang phục vụ bài nộp.",
      SAFE_READ_TITLE: "Mặt phẳng quan sát an toàn",
      SAFE_READ_DESCRIPTION:
        "Trang này chỉ hiển thị trạng thái đã được lược bỏ dữ liệu nhạy cảm. Kích hoạt và ngừng capability tiếp tục qua CLI có kiểm toán.",
      REFRESH: "Làm mới",
      REFRESHING: "Đang đồng bộ...",
      UPDATED_AT: (value: string) => `Ảnh chụp lúc ${value}`,
      LOAD_ERROR_TITLE: "Không thể tải trạng thái judge",
      LOAD_ERROR_DESCRIPTION:
        "Dữ liệu vận hành đang tạm thời không khả dụng. Hãy thử lại mà không cần tải lại trang.",
      QUEUE_TITLE: "Hàng đợi bền vững",
      QUEUE_DESCRIPTION:
        "Ảnh chụp nhất quán của toàn bộ vòng đời job trong local Forge queue.",
      READY: "Sẵn sàng",
      LEASED: "Đang thực thi",
      RETRY_WAIT: "Chờ thử lại",
      DONE: "Hoàn tất",
      CANCELLED: "Đã hủy",
      DEAD: "Dead letter",
      ACTIONABLE_JOBS: "Job cần xử lý",
      OLDEST_READY: "Job chờ lâu nhất",
      NO_WAITING_JOB: "Không có job đang chờ",
      WORKERS_TITLE: "Worker đang ghi nhận",
      WORKERS_DESCRIPTION:
        "Heartbeat, quyền nhận job và dung lượng của từng tiến trình worker.",
      WORKERS_EMPTY_TITLE: "Chưa có worker",
      WORKERS_EMPTY_DESCRIPTION:
        "Chưa có worker nào đăng ký trong cơ sở dữ liệu vận hành.",
      WORKER: "Worker",
      HEARTBEAT: "Heartbeat",
      CAPACITY: "Dung lượng",
      EVIDENCE: "Bằng chứng capability",
      BOOT: "Boot",
      CLAIMING: "Đang nhận job",
      DRAINING: "Đang drain",
      STALE: "Mất heartbeat",
      FRESH: "Ổn định",
      FREE_OF_TOTAL: (free: number, total: number) => `${free}/${total} slot trống`,
      EVIDENCE_COUNT: (count: number) => `${count} bằng chứng đạt`,
      NO_PROBE: "Chưa có probe đạt",
      ACTIVATIONS_TITLE: "Runtime capability",
      ACTIVATIONS_DESCRIPTION:
        "Release, runtime và checker đã được phê duyệt bằng bằng chứng từ worker.",
      ACTIVATIONS_EMPTY_TITLE: "Chưa có capability",
      ACTIVATIONS_EMPTY_DESCRIPTION:
        "Chưa có runtime capability nào được kích hoạt hoặc lưu trong lịch sử.",
      RUNTIME: "Runtime",
      CHECKER: "Checker",
      RELEASE: "Release",
      PROFILE: "Profile",
      ACTIVATED_AT: "Kích hoạt",
      ACTIVE: "Đang hoạt động",
      RETIRED: "Đã ngừng",
      ACTIVE_ACTIVATIONS: "Capability hoạt động",
      HEALTHY_WORKERS: "Worker ổn định",
      TOTAL_CAPACITY: "Tổng slot",
      SNAPSHOT_LABEL: "Ảnh chụp vận hành judge",
    },

    OBSERVABILITY: {
      TITLE: "Quan Sát Hệ Thống",
      SUBTITLE:
        "Điều khiển mức log debug có thời hạn cho tiến trình API mà không làm lộ dữ liệu nhạy cảm.",
      TRUST_LABEL: "Debug có hàng rào an toàn",
      TRUST_DESCRIPTION:
        "Mỗi lần bật hoặc kết thúc sớm đều yêu cầu quyền riêng, xác thực lại gần đây, lý do vận hành và bản ghi kiểm toán bất biến. Hệ thống tự trở về mức log cấu hình khi hết hạn.",
      REFRESH: "Làm mới",
      REFRESHING: "Đang đồng bộ...",
      LOAD_ERROR_TITLE: "Không thể tải trạng thái quan sát",
      LOAD_ERROR_DESCRIPTION:
        "Control plane quan sát đang tạm thời không khả dụng. Mức log của tiến trình không bị thay đổi.",
      STALE_SNAPSHOT:
        "Chưa thể làm mới; trạng thái gần nhất vẫn được hiển thị. Hãy đồng bộ lại trước khi thao tác.",
      STATUS_TITLE: "Cửa sổ debug hiện tại",
      STATUS_DESCRIPTION:
        "Projection an toàn của mức log tạm thời trên tiến trình API.",
      ACTIVE: "Đang bật debug",
      INACTIVE: "Đang ở mức nền",
      COMPONENT: "Thành phần",
      COMPONENT_API: "API",
      COMPONENT_LABEL: (component: string) =>
        component === "api" ? "API" : component,
      LEVEL: "Mức log",
      LEVEL_DEBUG: "Debug tạm thời",
      LEVEL_BASELINE: "Mức cấu hình",
      STARTED_AT: "Bắt đầu",
      EXPIRES_AT: "Tự kết thúc",
      ACTIVATE_TITLE: "Bật debug tạm thời",
      ACTIVATE_DESCRIPTION:
        "Chỉ dùng trong khoảng ngắn để truy vết sự cố theo CID. Không ghi mật khẩu, token, source code hoặc payload nhạy cảm.",
      DEACTIVATE_TITLE: "Kết thúc cửa sổ debug",
      DEACTIVATE_DESCRIPTION:
        "Khôi phục mức log cấu hình ngay, không cần chờ hết thời hạn.",
      DURATION_LABEL: "Thời lượng",
      DURATION_OPTION: (seconds: number) =>
        `${Math.max(1, Math.round(seconds / 60))} phút`,
      REASON_LABEL: "Lý do kiểm toán",
      ACTIVATE_REASON_PLACEHOLDER:
        "Mô tả sự cố hoặc CID cần truy vết...",
      DEACTIVATE_REASON_PLACEHOLDER:
        "Nêu lý do kết thúc cửa sổ debug sớm...",
      REASON_HELP: (minimum: number, maximum: number) =>
        `Tối thiểu ${minimum} ký tự, tối đa ${maximum} byte UTF-8; không chứa ký tự điều khiển.`,
      READ_ONLY:
        "Bạn có quyền xem trạng thái nhưng chưa được cấp quyền thay đổi mức log.",
      ACTIVATE: "Bật debug",
      DEACTIVATE: "Kết thúc debug",
      COMMAND_PENDING: "Đang áp dụng...",
      ACTIVATE_SUCCESS: "Đã bật debug tạm thời cho tiến trình API.",
      DEACTIVATE_SUCCESS: "Đã khôi phục mức log cấu hình.",
      REAUTHENTICATION_REQUIRED:
        "Thao tác yêu cầu một lần xác thực quản trị gần đây.",
      CONFLICT:
        "Cửa sổ debug vừa thay đổi. Trạng thái mới nhất đang được đồng bộ.",
      COMMAND_ERROR:
        "Không thể thay đổi mức log. Tiến trình vẫn giữ trạng thái an toàn trước đó.",
    },

    OPERATIONS: {
      TITLE: "Tác Vụ Nền",
      SUBTITLE:
        "Theo dõi tiến độ và điều khiển an toàn các công việc dài hạn trên toàn hệ thống.",
      TRUST_LABEL: "Control plane bền vững",
      TRUST_DESCRIPTION:
        "Mỗi batch có checkpoint, lease và receipt bất biến; request gốc cùng lease token không bao giờ được gửi ra trình duyệt.",
      REFRESH: "Làm mới",
      REFRESHING: "Đang đồng bộ...",
      UPDATED_AT: (value: string) => `Cập nhật lúc ${value}`,
      FILTERS_TITLE: "Bộ lọc tác vụ",
      FILTERS_DESCRIPTION:
        "Thu hẹp theo trạng thái hoặc loại executor do máy chủ công bố.",
      STATUS_LABEL: "Trạng thái",
      ALL_STATUSES: "Tất cả trạng thái",
      KIND: "Loại tác vụ",
      KIND_PLACEHOLDER: "VD: notification.campaign",
      KIND_HINT:
        "Dùng chính xác mã loại hiển thị trong danh sách; để trống để xem tất cả.",
      KIND_LABELS: {
        NOTIFICATION_CAMPAIGN: "Chiến dịch thông báo",
        CONTEST_RATING_RERATING: "Tính lại rating lịch sử",
		CULTIVATION_RANKING_RECONCILIATION: "Đối soát bảng xếp hạng tu luyện",
        IDENTITY_ATTRIBUTE_VALUE_MIGRATION: "Di chuyển giá trị thuộc tính hồ sơ",
      },
      KIND_INVALID:
        "Mã loại bắt đầu bằng chữ thường và chỉ gồm chữ, số, dấu chấm, gạch ngang hoặc gạch dưới.",
      APPLY_FILTERS: "Áp dụng bộ lọc",
      RESET_FILTERS: "Xóa bộ lọc",
      LOAD_ERROR_TITLE: "Không thể tải tác vụ nền",
      LOAD_ERROR_DESCRIPTION:
        "Control plane đang tạm thời không khả dụng. Dữ liệu trên máy chủ không bị thay đổi; hãy thử đồng bộ lại.",
      EMPTY_TITLE: "Không tìm thấy tác vụ",
      EMPTY_DESCRIPTION:
        "Thử thay đổi bộ lọc hoặc làm mới để lấy trạng thái mới nhất.",
      CURRENT_PAGE: "Tóm tắt trang hiện tại",
      ACTIVE: "Đang xử lý",
      PAUSED: "Đã tạm dừng",
      COMPLETED: "Hoàn tất",
      FAILED: "Thất bại",
      TABLE_LABEL: "Danh sách tác vụ nền đang được quản trị",
      COLUMN_OPERATION: "Tác vụ",
      COLUMN_PROGRESS: "Tiến độ",
      COLUMN_STATE: "Trạng thái",
      COLUMN_ATTEMPTS: "Lần thử",
      COLUMN_TIMING: "Thời gian",
      COLUMN_ACTIONS: "Thao tác",
      CREATED_AT: "Tạo lúc",
      UPDATED_AT_LABEL: "Cập nhật",
      AVAILABLE_AT: "Có thể chạy từ",
      NOT_BEFORE: "Lịch chạy tối thiểu",
      NO_SCHEDULE_FLOOR: "Có thể chạy ngay",
      STARTED_AT: "Bắt đầu",
      FINISHED_AT: "Kết thúc",
      LEASE_UNTIL: "Lease đến",
      NO_LEASE: "Không có lease đang giữ",
      NOT_STARTED: "Chưa bắt đầu",
      NOT_FINISHED: "Chưa kết thúc",
      BATCH_SIZE: (value: number) => `Batch tối đa ${value.toLocaleString(APP_LOCALE)}`,
      PROGRESS_COUNT: (processed: number, total: number) =>
        `${processed.toLocaleString(APP_LOCALE)}/${total.toLocaleString(APP_LOCALE)} mục`,
      SUCCESS_FAILURE: (succeeded: number, failed: number) =>
        `${succeeded.toLocaleString(APP_LOCALE)} thành công · ${failed.toLocaleString(APP_LOCALE)} lỗi`,
      ATTEMPT_COUNT: (value: number) => `${value.toLocaleString(APP_LOCALE)} lần`,
      VERSION: (value: number) => `Phiên bản ${value}`,
      ACTOR: "Người khởi tạo",
      AUDIT_ID: "Audit ID",
      CORRELATION_ID: "Correlation ID",
      LAST_ERROR: "Mã lỗi an toàn",
      NO_ERROR: "Không ghi nhận lỗi",
      REQUEST_PENDING: "Đang chờ áp dụng yêu cầu điều khiển",
      VIEW_DETAILS: "Xem chi tiết",
      VIEW_DETAILS_ARIA: (kind: string) => `Xem chi tiết tác vụ ${kind}`,
      DETAIL_TITLE: "Chi Tiết Tác Vụ",
      DETAIL_DESCRIPTION:
        "Projection vận hành an toàn; payload, checkpoint và thông tin lease nhạy cảm được giữ kín ở máy chủ.",
      BATCH_HISTORY_TITLE: "Receipt từng batch",
      BATCH_HISTORY_DESCRIPTION:
        "Chỉ hiển thị tiến độ và mã lỗi an toàn. Payload nghiệp vụ, checkpoint và dữ liệu gốc không rời khỏi máy chủ.",
      BATCH_HISTORY_LIST_LABEL: "Danh sách receipt batch của tác vụ",
      BATCH_HISTORY_LOADING: "Đang tải receipt batch...",
      BATCH_HISTORY_LOADING_MORE: "Đang tải thêm batch...",
      BATCH_HISTORY_LOAD_MORE: "Tải thêm batch",
      BATCH_HISTORY_ERROR: "Chưa thể tải receipt batch.",
      BATCH_HISTORY_MORE_ERROR: "Chưa thể tải thêm receipt batch.",
      BATCH_HISTORY_EMPTY: "Tác vụ chưa commit batch nào.",
      BATCH_SEQUENCE: (sequence: number) => `Batch #${sequence.toLocaleString(APP_LOCALE)}`,
      BATCH_RESULT: (succeeded: number, failed: number) =>
        `${succeeded.toLocaleString(APP_LOCALE)} đạt · ${failed.toLocaleString(APP_LOCALE)} lỗi`,
      BATCH_FAILURES_LABEL: "Opaque reference và mã lỗi",
      CONTROL_RESTRICTED:
        "Bạn có quyền quan sát nhưng chưa được cấp quyền điều khiển tác vụ.",
      NO_CONTROL_AVAILABLE:
        "Tác vụ đã ở trạng thái kết thúc nên không còn thao tác điều khiển.",
      CONTROL: "Điều khiển",
      PAUSE: "Tạm dừng",
      RESUME: "Tiếp tục",
      CANCEL_OPERATION: "Hủy tác vụ",
      CONTROL_TITLE: (action: string) => `${action} tác vụ?`,
      PAUSE_DESCRIPTION:
        "Worker sẽ hoàn tất transaction batch đang giữ rồi chuyển tác vụ sang trạng thái tạm dừng.",
      RESUME_DESCRIPTION:
        "Tác vụ được đưa trở lại hàng chờ và tiếp tục từ checkpoint đã commit gần nhất.",
      CANCEL_DESCRIPTION:
        "Worker sẽ hoàn tất transaction batch đang giữ rồi dừng vĩnh viễn tác vụ này. Phần đã commit không bị hoàn tác.",
      REASON: "Lý do bắt buộc",
      REASON_PLACEHOLDER:
        "Nêu ngắn gọn lý do vận hành để lưu vào nhật ký kiểm toán...",
      REASON_HELP: (minimum: number, maximum: number) =>
        `Tối thiểu ${minimum} ký tự, tối đa ${maximum} byte UTF-8; không chứa ký tự điều khiển.`,
      REASON_INVALID: "Vui lòng nhập lý do hợp lệ trong giới hạn cho phép.",
      CONTROL_PENDING: "Đang gửi yêu cầu...",
      CONTROL_SUCCESS: (action: string) => `Đã gửi yêu cầu ${action}.`,
      CONTROL_REPLAYED:
        "Máy chủ đã trả lại đúng kết quả của lần gửi trước; không có thao tác trùng lặp.",
      CONTROL_CONFLICT:
        "Tác vụ vừa thay đổi phiên bản hoặc trạng thái. Danh sách mới nhất đã được đồng bộ; hãy kiểm tra trước khi thử lại.",
      CONTROL_ERROR:
        "Không thể hoàn tất yêu cầu điều khiển. Dữ liệu máy chủ vẫn an toàn; vui lòng thử lại.",
      PREVIOUS_PAGE: "Trang trước",
      NEXT_PAGE: "Trang sau",
      PAGE_NUMBER: (value: number) => `Trang ${value}`,
      STATUS: {
        PENDING: "Đang chờ",
        RUNNING: "Đang chạy",
        PAUSED: "Tạm dừng",
        COMPLETED: "Hoàn tất",
        FAILED: "Thất bại",
        CANCELLED: "Đã hủy",
      },
      REQUESTED_ACTION: {
        PAUSE: "Đang chờ tạm dừng",
        CANCEL: "Đang chờ hủy",
      },
    },

    NOTIFICATION_CAMPAIGNS: {
      EYEBROW: "Trung tâm truyền thông",
      TITLE: "Chiến Dịch Thông Báo",
      SUBTITLE:
        "Soạn bản tin, duyệt chính xác người nhận và phát qua tác vụ nền bền vững có thể truy vết.",
      CREATE: "Tạo chiến dịch",
      EDIT: "Sửa bản nháp",
      REVIEW_ACTION: "Duyệt & lên lịch",
      CANCEL_ACTION: "Hủy phát chiến dịch",
      CANCEL_ARIA: (title: string) => `Hủy phát chiến dịch ${title}`,
      IMMUTABLE: "Đã đóng băng",
      TRUST_LABEL: "Duyệt trước khi phát",
      TRUST_DESCRIPTION:
        "Nội dung, tập người nhận và lịch chạy được đóng băng cùng một revision. Phạm vi rộng yêu cầu xác thực lại; worker kiểm tra quyền nhận lần cuối trước khi tạo thông báo.",
      REFRESH: "Làm mới",
      REFRESHING: "Đang đồng bộ...",
      UPDATED_AT: (value: string) => `Cập nhật lúc ${value}`,
      FILTERS_TITLE: "Bộ lọc chiến dịch",
      FILTERS_DESCRIPTION:
        "Thu hẹp danh sách theo trạng thái vòng đời do máy chủ xác nhận.",
      STATUS_LABEL: "Trạng thái",
      ALL_STATUSES: "Tất cả trạng thái",
      APPLY_FILTERS: "Áp dụng bộ lọc",
      RESET_FILTERS: "Xóa bộ lọc",
      LOAD_ERROR_TITLE: "Không thể tải chiến dịch",
      LOAD_ERROR_DESCRIPTION:
        "Dữ liệu trên máy chủ vẫn an toàn. Hãy thử đồng bộ lại mà không cần tải lại trang.",
      EMPTY_TITLE: "Chưa có chiến dịch phù hợp",
      EMPTY_DESCRIPTION:
        "Tạo một bản nháp mới hoặc thay đổi bộ lọc để xem các chiến dịch khác.",
      CURRENT_PAGE: "Tóm tắt chiến dịch trên trang hiện tại",
      METRIC_DRAFTS: "Bản nháp",
      METRIC_QUEUED: "Đã lên lịch / đang phát",
      METRIC_DELIVERED: "Thông báo đã tạo",
      METRIC_FAILURES: "Lỗi cần chú ý",
      LIST_TITLE: "Danh sách chiến dịch",
      LIST_COUNT: (count: number) =>
        `${count.toLocaleString(APP_LOCALE)} chiến dịch trên trang`,
      TABLE_LABEL: "Danh sách chiến dịch thông báo",
      COLUMN_CAMPAIGN: "Chiến dịch",
      COLUMN_AUDIENCE: "Người nhận",
      COLUMN_DELIVERY: "Tiến độ phát",
      COLUMN_STATUS: "Trạng thái",
      COLUMN_TIMING: "Thời gian",
      COLUMN_ACTIONS: "Thao tác",
      REVISION: (value: number) => `Revision ${value}`,
      VERSION: (value: number) => `Phiên bản ${value}`,
      CREATED_AT: "Tạo lúc",
      UPDATED_AT_LABEL: "Cập nhật",
      SCHEDULED_FOR: "Lịch phát",
      NOT_SCHEDULED: "Chưa lên lịch",
      SCHEDULE_VALUE: (value: string) => `Phát lúc ${value}`,
      CREATED_VALUE: (value: string) => `Tạo lúc ${value}`,
      DELIVERED: "Đã xử lý",
      PROGRESS_COUNT: (processed: number, total: number) =>
        `${processed.toLocaleString(APP_LOCALE)}/${total.toLocaleString(APP_LOCALE)}`,
      VIEW_DETAILS: "Xem chi tiết",
      VIEW_DETAILS_ARIA: (title: string) => `Xem chi tiết chiến dịch ${title}`,
      EDIT_ARIA: (title: string) => `Sửa bản nháp chiến dịch ${title}`,
      REVIEW_ARIA: (title: string) => `Duyệt và lên lịch chiến dịch ${title}`,
      PREVIOUS_PAGE: "Trang trước",
      NEXT_PAGE: "Trang sau",
      PAGE_NUMBER: (value: number) => `Trang ${value}`,
      CREATE_SUCCESS: "Đã tạo bản nháp chiến dịch.",
      UPDATE_SUCCESS: "Đã tạo revision bản nháp mới.",
      SAVE_ERROR:
        "Không thể lưu chiến dịch. Nội dung đang nhập vẫn được giữ nguyên.",
      CONFLICT:
        "Chiến dịch vừa được cập nhật ở phiên khác. Dữ liệu mới nhất đã được đồng bộ; hãy kiểm tra lại trước khi lưu.",
      SCHEDULE_SUCCESS:
        "Đã đóng băng chiến dịch và tạo tác vụ phát thông báo.",
      SCHEDULE_REPLAYED:
        "Máy chủ trả lại đúng kết quả lần gửi trước; không tạo tác vụ trùng lặp.",
      CANCEL_SUCCESS:
        "Đã ghi nhận lệnh hủy chiến dịch và dừng các batch chưa bắt đầu.",
      CANCEL_REPLAYED:
        "Máy chủ trả lại đúng biên nhận hủy trước đó; không tạo thay đổi trùng lặp.",
      STATUS: {
        DRAFT: "Bản nháp",
        SCHEDULED: "Đã lên lịch",
        SENDING: "Đang phát",
        COMPLETED: "Hoàn tất",
        FAILED: "Thất bại",
        CANCELLED: "Đã hủy",
      },
      AUDIENCE: {
        EXPLICIT_USERS: "Danh sách người dùng",
        ROLE: "Theo vai trò",
        ALL_ACTIVE: "Toàn bộ tài khoản hoạt động",
        EXPLICIT_COUNT: (count: number) =>
          `${count.toLocaleString(APP_LOCALE)} người dùng được chọn`,
        ROLE_ID: (id: string) => `Vai trò ${id}`,
      },
      COMPOSER: {
        CREATE_TITLE: "Tạo Chiến Dịch",
        EDIT_TITLE: "Sửa Bản Nháp",
        DESCRIPTION:
          "Mỗi lần lưu tạo một revision nội dung bất biến. Chưa có thông báo nào được phát ở bước này.",
        COMPOSE_TAB: "Soạn nội dung",
        PREVIEW_TAB: "Xem trước",
        TITLE: "Tiêu đề",
        TITLE_PLACEHOLDER: "VD: Lịch bảo trì hệ thống cuối tuần",
        BODY: "Nội dung Markdown",
        BODY_PLACEHOLDER:
          "Viết nội dung rõ ràng, ngắn gọn và nêu chính xác hành động người dùng cần thực hiện...",
        BODY_HINT:
          "Hỗ trợ Markdown, bảng, code và công thức. Bản render cuối được máy chủ làm sạch và đóng băng theo revision.",
        ACTION_PATH: "Đường dẫn hành động",
        ACTION_PLACEHOLDER: "/notifications",
        ACTION_HINT:
          "Chỉ dùng đường dẫn nội bộ đã được máy chủ cho phép; không nhập URL ngoài hệ thống.",
        AUDIENCE_TITLE: "Phạm vi người nhận",
        AUDIENCE_DESCRIPTION:
          "Chọn phạm vi nhỏ nhất đáp ứng mục tiêu. Phạm vi rộng cần quyền riêng và xác thực lại khi duyệt.",
        AUDIENCE_KIND: "Loại phạm vi",
        ROLE: "Vai trò",
        ROLE_PLACEHOLDER: "Chọn một vai trò",
        ROLES_LOADING: "Đang tải vai trò...",
        ROLES_LOAD_ERROR:
          "Không thể tải danh sách vai trò. Hãy thử lại trước khi chọn phạm vi này.",
        RETRY_ROLES: "Tải lại danh sách vai trò",
        USER_IDS: "UUID người dùng",
        USER_IDS_PLACEHOLDER:
          "Mỗi dòng một UUID, hoặc phân tách bằng dấu phẩy",
        USER_IDS_HINT:
          "Danh sách được loại trùng, chuẩn hóa và kiểm tra lại trên máy chủ.",
        BROADCAST_WARNING:
          "Đây là phạm vi phát rộng. Hệ thống sẽ yêu cầu xác thực lại, tạo bản duyệt người nhận có thời hạn và kiểm tra quyền thêm một lần khi lên lịch.",
        REASON: "Lý do thay đổi",
        REASON_PLACEHOLDER:
          "Mô tả mục đích của nội dung hoặc thay đổi revision này...",
        BYTE_COUNT: (current: number, maximum: number) =>
          `${current.toLocaleString(APP_LOCALE)}/${maximum.toLocaleString(APP_LOCALE)} byte`,
        USER_COUNT: (current: number, maximum: number) =>
          `${current.toLocaleString(APP_LOCALE)}/${maximum.toLocaleString(APP_LOCALE)} người`,
        LOCAL_PREVIEW: "Bản xem trước cục bộ",
        UNTITLED: "Chiến dịch chưa có tiêu đề",
        PREVIEW_EMPTY:
          "Nhập tiêu đề hoặc nội dung để xem bố cục trước khi lưu revision.",
        INVALID:
          "Tiêu đề, nội dung và lý do là bắt buộc và phải nằm trong giới hạn byte.",
        AUDIENCE_INVALID:
          "Phạm vi người nhận chưa hợp lệ. Hãy chọn vai trò hoặc nhập ít nhất một UUID đúng định dạng.",
        SAVE: "Lưu revision",
        SAVING: "Đang lưu...",
      },
      REVIEW: {
        TITLE: "Duyệt & Lên Lịch",
        DESCRIPTION:
          "Tạo ảnh chụp người nhận, kiểm tra lại quyền và chỉ phát đúng revision vừa duyệt.",
        IMMUTABLE_REVISION: "Revision sẽ được đóng băng",
        SCHEDULE_MODE: "Thời điểm phát",
        SEND_NOW: "Phát ngay khi worker sẵn sàng",
        SEND_LATER: "Lên lịch thời điểm cụ thể",
        SCHEDULED_FOR: "Ngày giờ phát",
        IMMEDIATE_HINT: "Tác vụ có thể được worker nhận ngay sau khi xác nhận.",
        SCHEDULE_INVALID:
          "Thời điểm phát phải ở tương lai và không quá 365 ngày.",
        REAUTH_TITLE: "Xác thực lại cho phạm vi rộng",
        REAUTH_DESCRIPTION:
          "Mật khẩu chỉ được gửi đến endpoint xác thực phiên, không được lưu cùng chiến dịch hoặc token xác nhận.",
        CURRENT_PASSWORD: "Mật khẩu hiện tại",
        PASSWORD_PLACEHOLDER: "Nhập mật khẩu để tiếp tục",
        PASSWORD_REQUIRED:
          "Nhập mật khẩu hiện tại trước khi duyệt phạm vi rộng.",
        RUN_REVIEW: "Kiểm tra người nhận",
        REVIEWING: "Đang kiểm tra quyền và người nhận...",
        REVIEW_ERROR:
          "Không thể hoàn tất bản duyệt. Quyền, phiên đăng nhập hoặc dữ liệu người nhận có thể vừa thay đổi.",
        READY_TITLE: "Bản duyệt đã sẵn sàng",
        RECIPIENT_COUNT: (count: number) =>
          `${count.toLocaleString(APP_LOCALE)} người nhận đủ điều kiện tại thời điểm kiểm tra.`,
        ESTIMATED_AT: "Kiểm tra lúc",
        CONFIRMATION_EXPIRES: "Xác nhận hết hạn",
        SAMPLE: "Mẫu người nhận",
        REVIEW_AGAIN: "Kiểm tra lại",
        CONFIRMATION_EXPIRED:
          "Bản duyệt đã hết hạn. Hãy kiểm tra lại người nhận để tạo xác nhận mới.",
        REASON: "Lý do phát",
        REASON_PLACEHOLDER:
          "Nêu lý do phát chiến dịch để lưu trong nhật ký kiểm toán...",
        REASON_INVALID:
          "Lý do phát là bắt buộc và phải nằm trong giới hạn byte.",
        CONFIRM: "Đóng băng & lên lịch",
        SCHEDULING: "Đang tạo tác vụ...",
        SCHEDULE_ERROR:
          "Không thể lên lịch. Bản nháp hoặc phạm vi người nhận có thể vừa thay đổi; hãy duyệt lại trước khi thử tiếp.",
      },
      CANCEL: {
        TITLE: "Hủy Phát Chiến Dịch",
        DESCRIPTION:
          "Hành động này giữ nguyên lịch sử và kết quả đã phát, đồng thời chặn mọi batch chưa bắt đầu.",
        WARNING_TITLE: "Thao tác vận hành nhạy cảm",
        WARNING_DESCRIPTION:
          "Thông báo đã tạo không bị thu hồi. Phiên bản campaign và operation được kiểm tra đồng thời để không hủy nhầm dữ liệu vừa thay đổi.",
        CAMPAIGN_VERSION: (value: number) => `Campaign version ${value}`,
        OPERATION_VERSION: (value: number) => `Operation version ${value}`,
        CURRENT_PASSWORD: "Mật khẩu hiện tại",
        PASSWORD_PLACEHOLDER: "Nhập mật khẩu để xác thực lại",
        PASSWORD_REQUIRED: "Nhập mật khẩu hiện tại trước khi hủy chiến dịch.",
        REASON: "Lý do hủy",
        REASON_PLACEHOLDER:
          "Mô tả nguyên nhân cần dừng phát để lưu vào nhật ký kiểm toán...",
        REASON_INVALID:
          "Lý do hủy là bắt buộc và phải nằm trong giới hạn byte.",
        CONFLICT:
          "Chiến dịch hoặc tác vụ vừa thay đổi. Dữ liệu mới nhất đang được đồng bộ để bạn kiểm tra lại.",
        FORBIDDEN:
          "Phiên xác thực hoặc quyền hiện tại không còn đủ để hủy chiến dịch.",
        ERROR:
          "Không thể hủy chiến dịch lúc này. Không có thay đổi một phần nào được ghi nhận.",
        DISMISS: "Giữ chiến dịch",
        CONFIRM: "Xác thực & hủy phát",
        CANCELLING: "Đang hủy an toàn...",
      },
      DETAIL: {
        DESCRIPTION:
          "Nội dung, phạm vi, tiến độ và định danh vận hành của chiến dịch.",
        OVERVIEW_TAB: "Tổng quan",
        RECIPIENTS_TAB: "Người nhận",
        DELIVERY_PROGRESS: "Tiến độ phát",
        PROGRESS_COUNT: (processed: number, total: number) =>
          `${processed.toLocaleString(APP_LOCALE)}/${total.toLocaleString(APP_LOCALE)} người đã xử lý`,
        CONTENT: "Nội dung đã render",
        ACTION_PATH: "Đường dẫn hành động",
        AUDIENCE: "Phạm vi đã chọn",
        SELECTOR: "Bộ chọn người nhận",
        AUTHOR: "Người tạo revision",
        TIMELINE: "Dòng thời gian",
        STARTED_AT: "Bắt đầu phát",
        COMPLETED_AT: "Hoàn tất",
        CANCELLED_AT: "Đã hủy lúc",
        CANCELLED_BY: "Người hủy",
        CANCELLED_BY_VALUE: (value: string) => `Người hủy: ${value}`,
        CANCEL_REASON: "Lý do hủy",
        CANCELLATION_PENDING:
          "Worker đã nhận yêu cầu hủy và sẽ xác nhận tại biên batch an toàn.",
        IDENTIFIERS: "Định danh truy vết",
        CAMPAIGN_ID: "Campaign ID",
        REVISION_ID: "Revision ID",
        OPERATION_ID: "Operation ID",
        OPEN_OPERATION: "Mở control plane tác vụ",
        IMMUTABLE_NOTICE:
          "Chiến dịch đã lên lịch là dữ liệu chỉ đọc. Nội dung và tập người nhận đã được đóng băng để bảo toàn kiểm toán.",
      },
      RECIPIENT: {
        TITLE: "Kết quả theo người nhận",
        DESCRIPTION:
          "Projection an toàn theo từng tài khoản; payload nội dung và dữ liệu xác thực không xuất hiện tại đây.",
        FILTER: "Lọc trạng thái người nhận",
        ALL_STATUSES: "Tất cả trạng thái",
        LOAD_ERROR: "Không thể tải kết quả phát đến người nhận.",
        EMPTY: "Không có người nhận trong trạng thái đã chọn.",
        TABLE_LABEL: "Kết quả phát thông báo theo người nhận",
        USER: "Người dùng",
        STATE: "Trạng thái",
        RESULT: "Thông báo / mã kết quả",
        ATTEMPTS: "Số lần thử",
        PROCESSED_AT: "Xử lý lúc",
        STATUS: {
          PENDING: "Đang chờ",
          CREATED: "Đã tạo",
          SKIPPED: "Bỏ qua",
          FAILED: "Thất bại",
        },
      },
    },

    AUDIT: {
      TITLE: "Nhật Ký Kiểm Toán",
      SUBTITLE:
        "Truy vết các thay đổi nhạy cảm và hoạt động quản trị từ dữ liệu bất biến của hệ thống.",
      TRUST_LABEL: "Nhật ký bất biến",
      TRUST_DESCRIPTION:
        "Dữ liệu hiển thị đã được máy chủ loại bỏ thông tin nhạy cảm trước khi lưu.",
      REFRESH: "Làm mới",
      REFRESHING: "Đang làm mới...",
      UPDATED_AT: (value: string) => `Cập nhật lúc ${value}`,
      FILTERS_TITLE: "Bộ lọc truy vết",
      FILTERS_DESCRIPTION:
        "Kết hợp định danh, hành động và khoảng thời gian để thu hẹp sự kiện cần kiểm tra.",
      ACTOR_USER_ID: "Actor User ID",
      ACTOR_USER_ID_PLACEHOLDER: "Nhập UUID người thực hiện",
      ACTION: "Hành động",
      ACTION_PLACEHOLDER: "VD: identity.role.replace",
      RESOURCE: "Tài nguyên",
      RESOURCE_PLACEHOLDER: "VD: role",
      RESOURCE_ID: "Target ID",
      RESOURCE_ID_PLACEHOLDER: "Nhập định danh đối tượng",
      CORRELATION_ID: "Correlation ID",
      CORRELATION_ID_PLACEHOLDER: "Nhập CID chính xác",
      FROM: "Từ thời điểm",
      TO: "Đến thời điểm",
      APPLY_FILTERS: "Áp dụng bộ lọc",
      RESET_FILTERS: "Xóa bộ lọc",
      ACTOR_USER_ID_INVALID: "Actor User ID phải là UUID hợp lệ.",
      ACTION_INVALID:
        "Hành động chỉ được chứa chữ cái, chữ số và các ký tự . _ : -",
      RESOURCE_INVALID:
        "Tài nguyên chỉ được chứa chữ cái, chữ số và các ký tự . _ : -",
      RESOURCE_ID_INVALID: "Target ID vượt quá giới hạn cho phép.",
      CORRELATION_ID_INVALID:
        "Correlation ID chỉ được chứa chữ cái, chữ số và các ký tự . _ : -",
      FILTER_TOO_LONG: "Giá trị bộ lọc vượt quá giới hạn cho phép.",
      TIME_RANGE_INVALID:
        "Khoảng thời gian không hợp lệ hoặc thời điểm bắt đầu nằm sau thời điểm kết thúc.",
      LOAD_ERROR_TITLE: "Không thể tải nhật ký kiểm toán",
      LOAD_ERROR_DESCRIPTION:
        "Dữ liệu truy vết tạm thời không khả dụng. Hãy thử lại mà không cần tải lại trang.",
      EMPTY_TITLE: "Không tìm thấy sự kiện",
      EMPTY_DESCRIPTION:
        "Thử nới rộng khoảng thời gian, thay đổi bộ lọc hoặc làm mới dữ liệu.",
      RESULT_SUMMARY: "Tóm tắt trang hiện tại",
      EVENTS_ON_PAGE: "Sự kiện",
      ACTORS_ON_PAGE: "Actors",
      RESOURCES_ON_PAGE: "Tài nguyên",
      TRACE_IDS_ON_PAGE: "Trace IDs",
      TABLE_LABEL: "Danh sách sự kiện kiểm toán",
      COLUMN_EVENT: "Sự kiện",
      COLUMN_ACTOR: "Người thực hiện",
      COLUMN_TRACE: "Truy vết",
      COLUMN_TIME: "Thời điểm",
      COLUMN_ACTIONS: "Chi tiết",
      SYSTEM_ACTOR: "Hệ thống",
      TARGET_LABEL: "Target",
      OPEN_DETAILS: "Xem chi tiết",
      OPEN_DETAILS_ARIA: (action: string) => `Xem chi tiết sự kiện ${action}`,
      PREVIOUS_PAGE: "Trang trước",
      NEXT_PAGE: "Trang tiếp",
      PAGE_NUMBER: (page: number) => `Trang ${page}`,
      DETAIL_TITLE: "Chi Tiết Sự Kiện",
      DETAIL_DESCRIPTION:
        "Ngữ cảnh truy vết và thay đổi đã được loại bỏ dữ liệu nhạy cảm tại máy chủ.",
      EVENT_ID: "Event ID",
      OCCURRED_AT: "Xảy ra lúc",
      REASON: "Lý do",
      BEFORE: "Trước thay đổi",
      AFTER: "Sau thay đổi",
      NO_DIFF: "Không có dữ liệu thay đổi để hiển thị.",
      COPY_CID: "Sao chép Correlation ID",
      COPY_CID_SUCCESS: "Đã sao chép Correlation ID.",
      COPY_CID_ERROR:
        "Không thể sao chép tự động. Hãy chọn và sao chép Correlation ID thủ công.",
      RECENT_TITLE: "Hoạt Động Gần Đây",
      RECENT_DESCRIPTION:
        "Các sự kiện quản trị mới nhất mà tài khoản của bạn được phép xem.",
      VIEW_ALL: "Xem toàn bộ nhật ký",
      RECENT_LOADING: "Đang tải hoạt động gần đây...",
      RECENT_ERROR: "Không thể tải hoạt động gần đây.",
      RECENT_EMPTY: "Chưa có sự kiện kiểm toán trong phạm vi hiện tại.",
    },

    // Users page
    USERS_TITLE: "Người Dùng",
    USERS_SUBTITLE: "Quản lý tài khoản người dùng trong hệ thống",
    USERS_CREATE: "Mời Người Dùng",
    USERS_EMPTY: "Chưa có người dùng nào",
    USERS_SEARCH_PLACEHOLDER: "Tìm theo tên đăng nhập...",
    USERS_LOADING: "Đang tải danh sách người dùng",
    USERS_LOAD_ERROR: "Không thể tải danh sách người dùng. Vui lòng thử lại.",
    USERS_REFRESH_ERROR:
      "Không thể làm mới danh sách. Dữ liệu gần nhất vẫn được giữ lại.",
    USERS_ROLE_CATALOG_LOADING:
      "Đang tải catalog vai trò. Các thao tác phân quyền tạm thời chưa khả dụng.",
    USERS_ROLE_CATALOG_ERROR:
      "Không thể tải catalog vai trò. Danh sách người dùng vẫn khả dụng.",
    USERS_ROLE_CATALOG_EMPTY:
      "Hệ thống chưa có vai trò khả dụng; không thể tạo người dùng hoặc chỉnh sửa phân quyền.",
    USERS_COL_ID: "ID",
    USERS_COL_USERNAME: "Tên đăng nhập",
    USERS_COL_STATUS: "Trạng thái",
    USERS_COL_ROLE: "Vai Trò",
    USERS_COL_CREATED: "Ngày Tạo",
    USERS_EDIT_ROLE_TITLE: "Đổi Vai Trò",
    USERS_EDIT_ROLE_DESC: (username: string) => `Cập nhật vai trò cho ${username}`,
    USERS_EDIT_ROLE_SUCCESS: "Đã cập nhật vai trò",
    USERS_EDIT_ROLE_ERROR: "Cập nhật thất bại",
    USERS_FORM_ROLES: "Vai trò",
    USERS_ROLE_REQUIRED: "Chọn ít nhất một vai trò.",
    USERS_INVITATION: {
      TITLE: "Mời người dùng mới",
      DESCRIPTION:
        "Chuẩn bị tài khoản, phân quyền và hồ sơ ban đầu; người nhận tự đặt mật khẩu qua liên kết bảo mật.",
      SECURITY_TITLE: "Quản trị viên không tạo mật khẩu thay người dùng",
      SECURITY_DESCRIPTION:
        "Judgify chỉ gửi liên kết dùng một lần tới email khôi phục. Tài khoản chưa thể đăng nhập cho đến khi người nhận tự đặt mật khẩu.",
      USERNAME: "Tên đăng nhập",
      USERNAME_PLACEHOLDER: "Ví dụ: user01",
      USERNAME_INVALID: "Tên đăng nhập chưa đúng định dạng yêu cầu.",
      RECOVERY_EMAIL: "Email nhận lời mời",
      RECOVERY_EMAIL_PLACEHOLDER: "tenban@example.com",
      RECOVERY_EMAIL_HELP:
        "Địa chỉ này sẽ trở thành email khôi phục đã xác minh sau khi người nhận chấp nhận lời mời.",
      RECOVERY_EMAIL_INVALID: "Nhập một địa chỉ email hợp lệ.",
      ROLES: "Vai trò ban đầu",
      ROLES_REQUIRED: "Chọn ít nhất một vai trò.",
      REASON: "Lý do mời tài khoản",
      REASON_PLACEHOLDER:
        "Mô tả nhu cầu cấp tài khoản để lưu trong nhật ký kiểm toán...",
      REASON_HELP: "Lý do chỉ xuất hiện trong nhật ký quản trị, không gửi qua email.",
      REASON_INVALID: "Nhập lý do hợp lệ trước khi kiểm tra lời mời.",
      REASON_COUNT: (current: number, maximum: number) =>
        `${current}/${maximum} ký tự`,
      REVIEW: "Kiểm tra lời mời",
      REVIEWING: "Đang kiểm tra...",
      SERVER_REVIEWED: "Lời mời đã được máy chủ kiểm tra",
      CONFIRMATION_EXPIRES: (value: string) =>
        `Bản xác nhận quản trị có hiệu lực đến ${value}`,
      ACCOUNT: "Tài khoản",
      DESTINATION: "Điểm nhận đã che",
      PROFILE_FIELDS: "Trường hồ sơ",
      REASON_REVIEWED: "Lý do đã ghi nhận",
      IMMUTABLE_NOTICE:
        "Lệnh chỉ áp dụng đúng tên đăng nhập, email, vai trò, hồ sơ, lý do và phiên quản trị đang hiển thị. Mọi chỉnh sửa đều phải được kiểm tra lại.",
      REAUTH_TITLE: "Xác minh lại danh tính",
      REAUTH_DESCRIPTION:
        "Phiên bảo mật đã quá thời hạn cho thao tác nhạy cảm. Nhập mật khẩu hiện tại để gửi đúng lời mời đã kiểm tra.",
      CURRENT_PASSWORD: "Mật khẩu hiện tại",
      PASSWORD_REQUIRED: "Nhập mật khẩu hiện tại để xác minh lại danh tính.",
      BACK: "Quay lại chỉnh sửa",
      SEND_INVITATION: "Gửi lời mời",
      SENDING: "Đang gửi...",
      PREVIEW_ERROR:
        "Chưa thể kiểm tra lời mời. Bản nháp của bạn vẫn được giữ nguyên.",
      APPLY_ERROR:
        "Chưa thể gửi lời mời. Có thể thử lại an toàn với cùng mã lệnh.",
      REAUTH_REQUIRED:
        "Cần xác minh lại danh tính trước khi gửi lời mời này.",
      REAUTH_ERROR:
        "Mật khẩu không đúng hoặc chưa thể xác minh lại phiên quản trị lúc này.",
      EXPIRED:
        "Bản xác nhận đã hết hạn. Quay lại để máy chủ kiểm tra lời mời lần nữa.",
      CONFLICT:
        "Dữ liệu liên quan đã thay đổi. Catalog mới nhất đã được tải lại; hãy kiểm tra bản nháp rồi thử lại.",
      SUCCESS: "Đã tạo tài khoản chờ và xếp lịch gửi lời mời.",
    },
    USERS_INVITATION_RESEND: {
      TITLE: "Gửi lại lời mời",
      DESCRIPTION: (username: string) =>
        `Thu hồi liên kết cũ và phát hành lời mời mới cho tài khoản “${username}”.`,
      EFFECT:
        "Liên kết đang còn hiệu lực sẽ bị vô hiệu hóa ngay khi lệnh hoàn tất. Chỉ liên kết mới nhất mới có thể kích hoạt tài khoản.",
      REASON: "Lý do gửi lại",
      REASON_PLACEHOLDER:
        "Ví dụ: người nhận báo liên kết cũ đã hết hạn hoặc không còn truy cập được...",
      REASON_HELP:
        "Lý do được lưu trong nhật ký kiểm toán và không xuất hiện trong email.",
      REASON_INVALID: "Nhập lý do hợp lệ trước khi kiểm tra lệnh gửi lại.",
      REASON_COUNT: (current: number, maximum: number) =>
        `${current}/${maximum} ký tự`,
      REVIEW: "Kiểm tra lệnh gửi lại",
      REVIEWING: "Đang kiểm tra...",
      SERVER_REVIEWED: "Lệnh gửi lại đã được máy chủ kiểm tra",
      CONFIRMATION_EXPIRES: (value: string) =>
        `Bản xác nhận quản trị có hiệu lực đến ${value}`,
      DESTINATION: "Điểm nhận đã che",
      CURRENT_EXPIRY: "Liên kết hiện tại hết hạn",
      VERSION: "Phiên bản lời mời",
      REASON_REVIEWED: "Lý do đã ghi nhận",
      IMMUTABLE_NOTICE:
        "Khi xác nhận, máy chủ chỉ áp dụng đúng tài khoản, phiên bản lời mời, lý do và phiên quản trị đang hiển thị. Liên kết cũ sẽ không thể dùng lại.",
      REAUTH_TITLE: "Xác minh lại danh tính",
      REAUTH_DESCRIPTION:
        "Phiên bảo mật đã quá thời hạn cho thao tác nhạy cảm. Nhập mật khẩu hiện tại để phát hành đúng lời mời đã kiểm tra.",
      CURRENT_PASSWORD: "Mật khẩu hiện tại",
      PASSWORD_REQUIRED: "Nhập mật khẩu hiện tại để xác minh lại danh tính.",
      BACK: "Quay lại chỉnh sửa",
      SEND: "Thu hồi và gửi lại",
      SENDING: "Đang phát hành...",
      PREVIEW_ERROR:
        "Chưa thể kiểm tra lệnh gửi lại. Lý do của bạn vẫn được giữ nguyên.",
      APPLY_ERROR:
        "Chưa thể phát hành lời mời mới. Có thể thử lại an toàn với cùng mã lệnh.",
      REAUTH_REQUIRED:
        "Cần xác minh lại danh tính trước khi phát hành lời mời mới.",
      REAUTH_ERROR:
        "Mật khẩu không đúng hoặc chưa thể xác minh lại phiên quản trị lúc này.",
      EXPIRED:
        "Bản xác nhận đã hết hạn. Quay lại để máy chủ kiểm tra trạng thái mới nhất.",
      CONFLICT:
        "Lời mời hoặc tài khoản vừa thay đổi ở nơi khác. Dữ liệu mới nhất đang được tải lại; hãy kiểm tra lại trước khi tiếp tục.",
      RATE_LIMITED:
        "Lời mời vừa được gửi gần đây. Hãy chờ hết thời gian giới hạn rồi thử lại.",
      SUCCESS:
        "Đã thu hồi liên kết cũ và xếp lịch gửi lời mời mới.",
      ACTION: (username: string) => `Gửi lại lời mời cho ${username}`,
    },
    USERS_ROLE_REASON_LABEL: "Lý do thay đổi vai trò",
    USERS_ROLE_REASON_PLACEHOLDER:
      "Mô tả ngắn gọn lý do cần thay đổi vai trò...",
    USERS_ROLE_REASON_HINT: (minimum: number, maximum: number) =>
      `Bắt buộc từ ${minimum} đến ${maximum} ký tự để phục vụ nhật ký kiểm toán.`,
    USERS_ROLE_REASON_COUNT: (current: number, maximum: number) =>
      `${current}/${maximum} ký tự`,
    USERS_ROLE_REASON_REQUIRED:
      "Nhập lý do hợp lệ trước khi lưu thay đổi vai trò.",
    USERS_ROLE_CONFLICT_RELOADED:
      "Vai trò đã được thay đổi ở phiên làm việc khác. Dữ liệu mới nhất đã được tải lại; lựa chọn và lý do của bạn vẫn được giữ để kiểm tra trước khi lưu lại.",
    USERS_ROLE_CONFLICT_RETRY:
      "Vai trò đã được thay đổi ở phiên làm việc khác. Không thể tải lại dữ liệu ngay bây giờ; lựa chọn và lý do của bạn vẫn được giữ.",
    USERS_ROLE_REVIEW: {
      REVIEW: "Kiểm tra thay đổi",
      REVIEWING: "Đang kiểm tra...",
      TITLE: "Xác nhận vai trò người dùng",
      DESCRIPTION: (username: string) =>
        `Kiểm tra bản thay đổi bất biến dành cho tài khoản “${username}” trước khi áp dụng.`,
      SERVER_REVIEWED: "Đã được máy chủ kiểm tra",
      CURRENT_REVISION: "Phiên bản hiện tại",
      RESULTING_REVISION: "Phiên bản sau thay đổi",
      ADDED_TITLE: "Vai trò sẽ được gán",
      REMOVED_TITLE: "Vai trò sẽ bị gỡ",
      NO_ADDITIONS: "Không gán thêm vai trò nào.",
      NO_REMOVALS: "Không gỡ vai trò nào.",
      REASON: "Lý do đã ghi nhận",
      EXPIRES_AT: (value: string) => `Xác nhận có hiệu lực đến ${value}`,
      IMMUTABLE_NOTICE:
        "Lệnh chỉ áp dụng đúng tài khoản, phiên bản, lý do và danh sách vai trò đang hiển thị. Mọi chỉnh sửa đều phải được kiểm tra lại.",
      BACK_TO_EDIT: "Quay lại chỉnh sửa",
      APPLY: "Xác nhận và áp dụng",
      APPLYING: "Đang áp dụng...",
      PREVIEW_ERROR:
        "Chưa thể kiểm tra thay đổi vai trò. Lựa chọn của bạn vẫn được giữ nguyên.",
      APPLY_ERROR:
        "Chưa thể áp dụng thay đổi. Có thể thử lại an toàn với cùng mã lệnh.",
      CONFIRMATION_EXPIRED:
        "Phiên xác nhận đã hết hạn. Dữ liệu mới nhất đang được tải lại để bạn kiểm tra lại.",
      EXPIRED:
        "Phiên xác nhận đã hết hạn. Quay lại chỉnh sửa để máy chủ kiểm tra lại thay đổi.",
      REAUTH_TITLE: "Xác minh lại danh tính",
      REAUTH_DESCRIPTION:
        "Phiên bảo mật hiện tại đã quá thời hạn cho thao tác nhạy cảm. Nhập mật khẩu hiện tại để tiếp tục với chính bản thay đổi này.",
      CURRENT_PASSWORD: "Mật khẩu hiện tại",
      CURRENT_PASSWORD_PLACEHOLDER: "Nhập mật khẩu hiện tại",
      PASSWORD_REQUIRED: "Nhập mật khẩu hiện tại để xác minh lại danh tính.",
      REAUTHENTICATING: "Đang xác minh...",
      REAUTH_ERROR:
        "Không thể xác minh lại danh tính. Kiểm tra mật khẩu rồi thử lại.",
      REAUTH_REQUIRED:
        "Cần xác minh lại danh tính trước khi áp dụng thay đổi vai trò này.",
    },
    USERS_LIFECYCLE: {
      TITLE: (action: "suspend" | "reactivate" | "delete") =>
        action === "suspend"
          ? "Tạm ngưng tài khoản"
          : action === "reactivate"
            ? "Kích hoạt lại tài khoản"
            : "Xoá tài khoản",
      DESCRIPTION: (
        action: "suspend" | "reactivate" | "delete",
        username: string,
      ) =>
        action === "suspend"
          ? `Kiểm tra việc tạm ngưng tài khoản “${username}” và thu hồi toàn bộ phiên đăng nhập.`
          : action === "reactivate"
            ? `Kiểm tra việc cho phép tài khoản “${username}” hoạt động trở lại.`
            : `Kiểm tra việc xoá tài khoản “${username}” và thu hồi quyền truy cập.`,
      TARGET_ACCOUNT: "Tài khoản chịu tác động",
      STATUS: (status: "invited" | "active" | "suspended" | "deleted") =>
        status === "invited"
          ? "Đang chờ nhận lời mời"
          : status === "active"
          ? "Đang hoạt động"
          : status === "suspended"
            ? "Đã tạm ngưng"
            : "Đã xoá",
      CURRENT_VERSION: (version: number) =>
        `Phiên bản bảo mật hiện tại: ${version}`,
      REASON: "Lý do quản trị",
      REASON_PLACEHOLDER:
        "Nêu rõ nguyên nhân, phạm vi ảnh hưởng và căn cứ thực hiện...",
      REASON_HELP:
        "Lý do sẽ được gắn bất biến với lệnh, nhật ký kiểm toán và sự kiện hệ thống.",
      REASON_INVALID: "Nhập lý do hợp lệ trước khi yêu cầu máy chủ kiểm tra.",
      REASON_COUNT: (current: number, maximum: number) =>
        `${current}/${maximum} ký tự`,
      EFFECT: (
        action: "suspend" | "reactivate" | "delete",
        currentStatus: "invited" | "active" | "suspended" | "deleted",
      ) =>
        action === "suspend"
          ? "Tất cả phiên đăng nhập của người dùng sẽ bị thu hồi ngay sau khi giao dịch hoàn tất. Dữ liệu tài khoản được giữ nguyên để có thể kích hoạt lại."
          : action === "reactivate"
            ? "Người dùng có thể đăng nhập lại sau khi thay đổi hoàn tất; các phiên cũ vẫn bị vô hiệu và không được khôi phục."
            : currentStatus === "invited"
              ? "Tài khoản chờ sẽ được xoá mềm; liên kết lời mời, email đang chờ và vai trò trực tiếp đều bị thu hồi. Người nhận không thể dùng lại liên kết cũ."
              : "Tài khoản sẽ được xoá mềm, toàn bộ phiên và liên kết vai trò trực tiếp bị thu hồi. Thao tác này không có nút hoàn tác trong giao diện.",
      REVIEW: "Kiểm tra thay đổi",
      REVIEWING: "Đang kiểm tra...",
      SERVER_REVIEWED: "Đã được máy chủ kiểm tra",
      EXPIRES_AT: (value: string) => `Xác nhận có hiệu lực đến ${value}`,
      STATUS_TRANSITION: "Trạng thái",
      VERSION_TRANSITION: "Phiên bản bảo mật",
      REASON_REVIEWED: "Lý do đã ghi nhận",
      IMMUTABLE_NOTICE: (action: "suspend" | "reactivate" | "delete") =>
        `Mã xác nhận chỉ áp dụng cho đúng tài khoản, phiên bản, lý do và thao tác ${
          action === "suspend"
            ? "tạm ngưng"
            : action === "reactivate"
              ? "kích hoạt lại"
              : "xoá"
        } đang hiển thị. Mọi thay đổi đều phải được kiểm tra lại.`,
      BACK: "Quay lại chỉnh sửa",
      APPLY: (action: "suspend" | "reactivate" | "delete") =>
        action === "suspend"
          ? "Xác nhận tạm ngưng"
          : action === "reactivate"
            ? "Xác nhận kích hoạt"
            : "Xác nhận xoá",
      APPLYING: "Đang áp dụng...",
      PREVIEW_ERROR:
        "Chưa thể kiểm tra thay đổi. Lý do của bạn vẫn được giữ nguyên.",
      APPLY_ERROR:
        "Chưa thể áp dụng thay đổi. Có thể thử lại an toàn với cùng mã lệnh.",
      CONFLICT_RELOADED:
        "Tài khoản đã thay đổi ở phiên làm việc khác. Dữ liệu mới nhất đã được tải lại; hãy kiểm tra lại lệnh.",
      CONFLICT_RETRY:
        "Tài khoản đã thay đổi ở phiên làm việc khác và chưa thể tải lại. Hãy thử làm mới danh sách.",
      EXPIRED:
        "Phiên xác nhận đã hết hạn. Quay lại để máy chủ kiểm tra lại thay đổi.",
      REAUTH_TITLE: "Xác minh lại danh tính",
      REAUTH_DESCRIPTION:
        "Thao tác nhạy cảm này yêu cầu phiên xác minh gần đây. Nhập mật khẩu hiện tại để tiếp tục với chính bản thay đổi đã kiểm tra.",
      CURRENT_PASSWORD: "Mật khẩu hiện tại",
      CURRENT_PASSWORD_PLACEHOLDER: "Nhập mật khẩu hiện tại",
      PASSWORD_REQUIRED: "Nhập mật khẩu hiện tại để xác minh danh tính.",
      REAUTHENTICATING: "Đang xác minh...",
      REAUTH_REQUIRED:
        "Cần xác minh lại danh tính trước khi áp dụng thay đổi này.",
      REAUTH_ERROR:
        "Không thể xác minh lại danh tính. Kiểm tra mật khẩu rồi thử lại.",
      SUCCESS: (action: "suspend" | "reactivate" | "delete") =>
        action === "suspend"
          ? "Đã tạm ngưng tài khoản"
          : action === "reactivate"
            ? "Đã kích hoạt lại tài khoản"
            : "Đã xoá tài khoản",
      SUSPEND_ACTION: (username: string) =>
        `Tạm ngưng tài khoản ${username}`,
      REACTIVATE_ACTION: (username: string) =>
        `Kích hoạt lại tài khoản ${username}`,
      DELETE_ACTION: (username: string) => `Xoá tài khoản ${username}`,
    },
    USERS_NO_ROLES: "Chưa gán vai trò",
    USERS_FINAL_SUPERADMIN_REQUIRED:
      "Không thể thu hồi quyền, tạm ngưng hoặc xoá quản trị viên toàn quyền đang hoạt động cuối cùng. Hãy kích hoạt hoặc cấp vai trò này cho một quản trị viên khác trước.",
    USERS_EDIT_ROLES_ACTION: (username: string) =>
      `Chỉnh sửa vai trò của ${username}`,
    ACTIONS: "Thao tác",

    // Roles & permissions page
    ROLES_TITLE: "Phân Quyền",
    ROLES_SUBTITLE: "Quản lý vai trò và ma trận quyền hạn truy cập tài nguyên",
    ROLES_DISPLAY_NAME: localizedRoleName,
    ROLES_DISPLAY_DESCRIPTION: localizedRoleDescription,
    ROLES_CREATE: "Tạo Vai Trò",
    ROLES_MANAGE_RESOURCES: "Quản Lý Tài Nguyên",
    ROLES_EDIT: "Chỉnh sửa",
    ROLES_DELETE: "Xoá",
    ROLES_ARCHIVE: "Lưu trữ",
    ROLES_DELETE_TITLE: "Xoá vai trò",
    ROLES_ARCHIVE_TITLE: "Lưu trữ vai trò",
    ROLES_DELETE_DESCRIPTION: (name: string) =>
      `Bạn có chắc muốn xoá vai trò “${name}”? Các liên kết phân quyền của vai trò sẽ bị thu hồi.`,
    ROLES_ARCHIVE_DESCRIPTION: (name: string) =>
      `Vai trò “${name}” sẽ được lưu trữ, không bị xoá vĩnh viễn. Hãy kiểm tra bản xem trước trước khi áp dụng.`,
    ROLES_DELETE_SUCCESS: "Đã xoá vai trò",
    ROLES_DELETE_ERROR: "Không thể xoá vai trò",
    ROLES_SAVE_PERMS: "Kiểm tra thay đổi",
    ROLES_CREATE_DIALOG_TITLE: "Tạo Vai Trò Mới",
    ROLES_CREATE_DIALOG_DESC: "Thêm vai trò mới để phân quyền cho người dùng.",
    ROLES_EDIT_DIALOG_TITLE: "Chỉnh Sửa Vai Trò",
    ROLES_EDIT_DIALOG_DESC: "Cập nhật thông tin vai trò trong hệ thống.",
    ROLES_FORM_NAME: "Tên vai trò",
    ROLES_FORM_NAME_PLACEHOLDER: "VD: Moderator, Reviewer...",
    ROLES_FORM_KEY: "Mã vai trò",
    ROLES_FORM_KEY_PLACEHOLDER: "VD: reviewer",
    ROLES_FORM_KEY_HINT:
      "Mã ổn định dùng trong chính sách; chỉ gồm chữ thường, số và dấu gạch dưới.",
    ROLES_SYSTEM_KEY_HINT: "Không thể đổi mã của vai trò hệ thống.",
    ROLES_IMMUTABLE_KEY_HINT:
      "Mã vai trò là định danh ổn định và không thể thay đổi sau khi tạo.",
    ROLES_FORM_DESCRIPTION: "Mô tả",
    ROLES_FORM_DESCRIPTION_PLACEHOLDER: "Phạm vi trách nhiệm của vai trò...",
    ROLES_CHANGE_REASON_LABEL: "Lý do thay đổi",
    ROLES_CHANGE_REASON_PLACEHOLDER:
      "Mô tả ngắn gọn lý do cần tạo hoặc cập nhật vai trò...",
    ROLES_EMPTY: "Chưa có vai trò nào",
    ROLES_EMPTY_DESCRIPTION: "Tạo vai trò đầu tiên để bắt đầu phân quyền.",
    ROLES_LIST_LABEL: "Danh sách vai trò",
    ROLES_SYSTEM_BADGE: "Hệ thống",
    ROLES_UNSAVED_BADGE: "Chưa lưu",
    ROLES_UNSAVED_CONFIRM:
      "Quyền chưa được lưu sẽ bị hủy. Bạn vẫn muốn chuyển vai trò?",
    ROLES_CREATE_SUCCESS: "Đã tạo vai trò",
    ROLES_UPDATE_SUCCESS: "Đã cập nhật vai trò",
    ROLES_ARCHIVE_SUCCESS: "Đã lưu trữ vai trò",
    ROLES_ARCHIVE_REASON:
      "Lưu trữ vai trò và thu hồi các liên kết phân quyền hiện có.",
    ROLES_METADATA_REVIEW: {
      UPDATE_TITLE: "Xác nhận cập nhật vai trò",
      ARCHIVE_TITLE: "Xác nhận lưu trữ vai trò",
      DESCRIPTION:
        "Máy chủ đã kiểm tra đúng dữ liệu và phiên bản trước khi áp dụng.",
      KEY: "Mã vai trò bất biến",
      VERSION_LABEL: "Phiên bản",
      VERSION: (before: number, after: number) => `${before} → ${after}`,
      BEFORE: "Trước thay đổi",
      AFTER: "Sau thay đổi",
      ROLE_PROJECTION: (name: string, description: string, status: string) => {
        const localizedStatus =
          status === "archived" ? "đã lưu trữ" : "đang hoạt động";
        const localizedDescription = description || "Không có mô tả";
        return `${name} — ${localizedDescription} (${localizedStatus})`;
      },
      REASON: "Lý do đã ghi nhận",
      EXPIRY_LABEL: "Thời hạn xác nhận",
      EXPIRES_AT: (value: string) => `Có hiệu lực đến ${value}`,
      ARCHIVE_EFFECT:
        "Khi áp dụng, vai trò sẽ ngừng hoạt động và mọi chính sách hoặc liên kết người dùng của vai trò sẽ được thu hồi trong cùng giao dịch.",
      REAUTH_TITLE: "Xác minh lại danh tính",
      REAUTH_DESCRIPTION:
        "Phiên bảo mật đã quá thời hạn cho thao tác nhạy cảm. Nhập mật khẩu hiện tại để tiếp tục với đúng bản thay đổi này.",
      PASSWORD: "Mật khẩu hiện tại",
      PASSWORD_PLACEHOLDER: "Nhập mật khẩu hiện tại",
      APPLY: "Xác nhận và áp dụng",
      APPLYING: "Đang áp dụng...",
      PREVIEW_ERROR:
        "Không thể kiểm tra thay đổi; bản nháp vẫn được giữ lại.",
      APPLY_ERROR:
        "Không thể áp dụng; bạn có thể thử lại an toàn với cùng mã lệnh.",
      CONFLICT:
        "Vai trò đã thay đổi. Dữ liệu mới nhất sẽ được tải lại để bạn kiểm tra và tạo bản xem trước mới.",
      CONFIRMATION_EXPIRED:
        "Phiên xác nhận đã hết hạn. Dữ liệu mới nhất sẽ được tải lại để bạn kiểm tra lại thay đổi.",
      REAUTH_ERROR:
        "Không thể xác minh lại danh tính. Kiểm tra mật khẩu rồi thử lại.",
    },
    ROLES_SAVE_ERROR: "Lưu vai trò thất bại",
    ROLES_PERMS_SUCCESS: "Đã lưu quyền hạn",
    ROLES_PERMS_ERROR: "Lưu quyền hạn thất bại",
    ROLES_POLICY_REVISION: (revision: number) =>
      `Phiên bản chính sách: ${revision}`,
    ROLES_POLICY_REASON_LABEL: "Lý do thay đổi",
    ROLES_POLICY_REASON_PLACEHOLDER:
      "Mô tả ngắn gọn vì sao cần thay đổi quyền hạn...",
    ROLES_POLICY_REASON_HINT: (minimum: number, maximum: number) =>
      `Bắt buộc từ ${minimum} đến ${maximum} ký tự để phục vụ nhật ký kiểm toán.`,
    ROLES_POLICY_REASON_COUNT: (current: number, maximum: number) =>
      `${current}/${maximum} ký tự`,
    ROLES_POLICY_REASON_REQUIRED:
      "Nhập lý do hợp lệ trước khi lưu thay đổi quyền hạn.",
    ROLES_POLICY_CONFLICT:
      "Chính sách đã được cập nhật ở phiên làm việc khác. Dữ liệu mới nhất đã được tải lại; vui lòng kiểm tra và thực hiện lại thay đổi.",
    ROLES_POLICY_REVIEW: {
      TITLE: "Xác nhận thay đổi quyền",
      DESCRIPTION: (role: string) =>
        `Kiểm tra bản thay đổi bất biến dành cho vai trò “${role}” trước khi áp dụng.`,
      SERVER_REVIEWED: "Đã được máy chủ kiểm tra",
      CURRENT_REVISION: "Phiên bản hiện tại",
      RESULTING_REVISION: "Phiên bản sau thay đổi",
      ADDED_COUNT: "Quyền được cấp",
      REMOVED_COUNT: "Quyền bị thu hồi",
      ADDED_TITLE: "Quyền sẽ được cấp",
      REMOVED_TITLE: "Quyền sẽ bị thu hồi",
      NO_ADDITIONS: "Không cấp thêm quyền nào.",
      NO_REMOVALS: "Không thu hồi quyền nào.",
      REASON: "Lý do đã ghi nhận",
      EXPIRES_AT: (value: string) => `Xác nhận có hiệu lực đến ${value}`,
      IMMUTABLE_NOTICE:
        "Nút xác nhận chỉ áp dụng đúng vai trò, phiên bản, lý do và tập quyền đang hiển thị. Bất kỳ thay đổi nào cũng cần kiểm tra lại.",
      APPLY: "Xác nhận và áp dụng",
      APPLYING: "Đang áp dụng...",
      PREVIEWING: "Đang kiểm tra...",
      PREVIEW_ERROR:
        "Chưa thể kiểm tra thay đổi quyền. Dữ liệu đang chỉnh sửa vẫn được giữ nguyên.",
      CONFIRMATION_EXPIRED:
        "Phiên xác nhận đã hết hạn. Dữ liệu mới nhất đang được tải lại để bạn kiểm tra lại.",
      REAUTH_TITLE: "Xác minh lại danh tính",
      REAUTH_DESCRIPTION:
        "Phiên bảo mật hiện tại đã quá thời hạn cho thao tác nhạy cảm. Nhập mật khẩu hiện tại để tiếp tục với chính bản thay đổi này.",
      CURRENT_PASSWORD: "Mật khẩu hiện tại",
      CURRENT_PASSWORD_PLACEHOLDER: "Nhập mật khẩu hiện tại",
      PASSWORD_REQUIRED: "Nhập mật khẩu hiện tại để xác minh lại danh tính.",
      REAUTHENTICATING: "Đang xác minh...",
      REAUTH_ERROR:
        "Không thể xác minh lại danh tính. Kiểm tra mật khẩu rồi thử lại.",
      APPLY_ERROR:
        "Chưa thể áp dụng thay đổi. Có thể thử lại an toàn với cùng mã lệnh.",
    },
    ROLES_PROTECTED_TITLE: "Chính sách hệ thống được bảo vệ",
    ROLES_PROTECTED_DESCRIPTION:
      "Vai trò này được backend đánh dấu bảo vệ để tránh làm mất quyền quản trị cốt lõi. Bạn có thể xem nhưng không thể sửa ma trận quyền.",
    ROLES_READ_ONLY_TITLE: "Chế độ chỉ xem",
    ROLES_READ_ONLY_DESCRIPTION:
      "Bạn có thể xem chính sách hiện tại nhưng cần quyền quản lý phân quyền để thay đổi ma trận quyền.",
    AUTHORIZATION_UNAVAILABLE: "Không thể tải hệ thống phân quyền",
    AUTHORIZATION_LOAD_ERROR: "Không thể tải danh sách vai trò.",
    AUTHORIZATION_CATALOG_LOADING: "Đang tải danh mục quyền...",
    AUTHORIZATION_CATALOG_ERROR:
      "Không thể tải danh mục quyền. Danh sách vai trò vẫn khả dụng và bạn có thể thử lại riêng phần này.",
    POLICIES_LOADING: "Đang tải chính sách quyền...",
    POLICIES_LOAD_ERROR: "Không thể tải quyền của vai trò.",
    RESOURCES_DIALOG_TITLE: "Quản Lý Tài Nguyên",
    RESOURCES_DIALOG_DESC: "Thêm, sửa hoặc xóa các tài nguyên được bảo vệ trong hệ thống phân quyền.",
    RESOURCES_COL_KEY: "Key",
    RESOURCES_COL_DESC: "Mô tả",
    RESOURCES_KEY_LABEL: "Key",
    RESOURCES_KEY_PLACEHOLDER: "VD: submission",
    RESOURCES_DESC_LABEL: "Mô tả",
    RESOURCES_DESC_PLACEHOLDER: "VD: Bài nộp của người dùng",
    RESOURCES_ADD: "Thêm",
    RESOURCES_SEARCH_PLACEHOLDER: "Tìm tài nguyên...",
    RESOURCES_EMPTY: "Không tìm thấy tài nguyên nào.",
    RESOURCES_CREATE_SUCCESS: "Đã thêm tài nguyên",
    RESOURCES_CREATE_ERROR: "Thêm tài nguyên thất bại",
    RESOURCES_UPDATE_SUCCESS: "Đã cập nhật tài nguyên",
    RESOURCES_UPDATE_ERROR: "Cập nhật tài nguyên thất bại",
    RESOURCES_DELETE_SUCCESS: "Đã xoá tài nguyên",
    RESOURCES_DELETE_ERROR: "Xoá tài nguyên thất bại",

    // Shared form field labels
    FIELDS: {
      NAME: "Tên",
      NAME_PLACEHOLDER: "Nhập tên...",
      DESCRIPTION: "Mô tả",
      DESCRIPTION_PLACEHOLDER: "Nhập mô tả (tuỳ chọn)...",
      CODE: "Mã code",
      CODE_PLACEHOLDER: "VD: FIRE",
      WEIGHT: "Trọng số",
      MIN_RATING: "Rating tối thiểu",
      MIN_EXP: "EXP tối thiểu",
      LEVEL_NUM: "Cấp độ",
      EXP_REWARD: "EXP thưởng",
      TYPE: "Loại",
      TYPE_ROOT_BONE: "Căn Cốt",
      TYPE_TALENT: "Thiên Phú",
      RARITY: "Phẩm chất",
      SELECT_RARITY_PLACEHOLDER: "Chọn phẩm chất...",
      ELEMENT: "Ngũ Hành",
      DIFFICULTY: "Độ khó",
      KEY: "Khóa",
      DATA_TYPE: "Kiểu dữ liệu",
      USER_ID: "ID người dùng",
      TRAIT_ID: "ID thiên phú",
      ELEMENT_ID: "ID ngũ hành",
      EXP: "EXP",
      TOTAL_EXP: "Tổng EXP",
      RATING: "Rating",
      SEARCH: "Tìm kiếm dữ liệu quản trị",
      UUID_INVALID: "Định danh phải là UUID hợp lệ.",
      SAFE_INTEGER_INVALID: (field: string) =>
        `${field} phải là số nguyên không âm không vượt quá ${Number.MAX_SAFE_INTEGER}.`,
    },

    LOOKUPS: {
      USER_SEARCH_LABEL: "Người dùng",
      USER_SEARCH_PLACEHOLDER: "Tìm theo username...",
      USER_RESULTS_LABEL: "Kết quả tìm người dùng",
      USER_SELECT_PLACEHOLDER: "Chọn người dùng...",
      USER_LOADING: "Đang tìm người dùng...",
      USER_LOAD_ERROR: "Không thể tải danh sách người dùng.",
      USER_EMPTY: "Không tìm thấy người dùng phù hợp.",
      TRAIT_LABEL: "Thiên phú",
      TRAIT_SELECT_PLACEHOLDER: "Chọn thiên phú...",
      ELEMENT_LABEL: "Ngũ hành",
      ELEMENT_SELECT_PLACEHOLDER: "Chọn ngũ hành...",
      CATALOG_EMPTY: "Danh mục chưa có lựa chọn khả dụng.",
    },

    // Attribute definitions
    ATTRIBUTE_DEFINITIONS: {
      TITLE: "Định Nghĩa Thuộc Tính",
      SUBTITLE: "Quản lý các thuộc tính hồ sơ được hệ thống hỗ trợ.",
      CREATE: "Tạo Thuộc Tính",
      SEARCH_PLACEHOLDER: "Tìm theo khóa...",
      EMPTY: "Chưa có định nghĩa thuộc tính nào",
      DIALOG_CREATE_TITLE: "Tạo Định Nghĩa Thuộc Tính",
      DIALOG_CREATE_DESC: "Thêm thuộc tính mới vào danh mục hồ sơ.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Định Nghĩa Thuộc Tính",
      DIALOG_EDIT_DESC: "Cập nhật kiểu dữ liệu hoặc mô tả thuộc tính.",
      COL_KEY: "Khóa",
      COL_DATA_TYPE: "Kiểu dữ liệu",
      COL_STATUS: "Vòng đời",
      COL_DESCRIPTION: "Mô tả",
      KEY_PLACEHOLDER: "VD: date_of_birth",
      KEY_IMMUTABLE: "Khóa là định danh ổn định và không thể thay đổi sau khi tạo.",
      DESCRIPTION_PLACEHOLDER: "Mô tả mục đích của thuộc tính...",
      KEY_INVALID: "Khóa phải có từ 2 đến 50 ký tự.",
      DESCRIPTION_INVALID: "Mô tả không được dài quá 1.024 ký tự.",
      LABEL: "Tên hiển thị",
      LABEL_PLACEHOLDER: "VD: Ngày sinh",
      DISPLAY_ORDER: "Thứ tự hiển thị",
      DISPLAY_ORDER_DESCRIPTION:
        "Số nhỏ hơn xuất hiện trước; các trường cùng thứ tự được xếp theo khóa ổn định.",
      DISPLAY_ORDER_INVALID: "Nhập số nguyên từ 0 đến 10.000.",
      VISIBILITY: "Phạm vi hiển thị",
      VISIBILITY_PUBLIC: "Công khai",
      VISIBILITY_PRIVATE: "Riêng tư",
      VISIBILITY_ADMIN: "Chỉ quản trị viên",
      USER_EDITABLE: "Người dùng được chỉnh sửa",
      REQUIRED_ONBOARDING: "Bắt buộc khi tạo tài khoản",
      REQUIRED_ONBOARDING_CODE_OWNED:
        "Thuộc tính hệ thống bắt buộc được khóa bởi catalog của phiên bản đang chạy.",
      VALIDATION: "Quy tắc validation",
      VALIDATION_HINT:
        "JSON object tối đa 8 KiB; hỗ trợ enum, min_length, max_length, pattern, minimum và maximum. Số phải ở dạng thập phân không exponent; số nguyên nằm trong miền an toàn của JavaScript.",
      REASON: "Lý do thay đổi",
      REASON_PLACEHOLDER:
        "Mô tả mục đích thay đổi để ghi vào nhật ký audit...",
      REASON_HELP: "Từ 3 đến 500 ký tự; không nhập dữ liệu nhạy cảm.",
      STATUS_ACTIVE: "Đang hoạt động",
      STATUS_ARCHIVED: "Đã lưu trữ",
      STATUS_DRAFT_READY: "Có revision chờ kích hoạt",
      HISTORY_ACTION: "Xem lịch sử revision",
      HISTORY_TITLE: "Lịch sử revision",
      HISTORY_DESCRIPTION: (key: string) =>
        `Kiểm tra toàn bộ revision của ${key} và chọn một bản cũ để khôi phục qua quy trình review an toàn.`,
      HISTORY_DESCRIPTION_FALLBACK:
        "Kiểm tra lịch sử revision và khôi phục qua quy trình review an toàn.",
      HISTORY_LIST_LABEL: "Danh sách lịch sử revision thuộc tính",
      HISTORY_LOADING: "Đang tải lịch sử revision...",
      HISTORY_LOADING_MORE: "Đang tải thêm...",
      HISTORY_LOAD_MORE: "Tải thêm revision",
      HISTORY_ERROR: "Chưa thể tải lịch sử revision.",
      HISTORY_MORE_ERROR: "Chưa thể tải thêm revision.",
      HISTORY_EMPTY: "Chưa có revision nào trong lịch sử.",
      HISTORY_ACTIVE: "Đang hiệu lực",
      HISTORY_DRAFT: "Bản nháp hiện tại",
      HISTORY_DATA_TYPE: "Kiểu dữ liệu",
      HISTORY_VISIBILITY: "Hiển thị",
      HISTORY_EDITABLE: "Người dùng chỉnh sửa",
      HISTORY_CREATED_AT: "Được tạo lúc",
      HISTORY_YES: "Có",
      HISTORY_NO: "Không",
      RESTORE_ACTION: "Khôi phục",
      RESTORE_TITLE: "Khôi phục revision thuộc tính",
      RESTORE_DESCRIPTION:
        "Revision đã chọn sẽ được kiểm tra như một ứng viên mới. Dữ liệu được phục hồi theo lô có checkpoint trước khi con trỏ active thay đổi.",
      ACTIVATE_ACTION: "Kiểm tra và kích hoạt revision",
      ARCHIVE_ACTION: "Kiểm tra và lưu trữ định nghĩa",
      ACTIVATE_TITLE: "Kích hoạt revision thuộc tính",
      ACTIVATE_DESCRIPTION:
        "Kiểm tra revision đang dùng, revision ứng viên và ảnh hưởng tới dữ liệu trước khi thay đổi schema hồ sơ.",
      ARCHIVE_TITLE: "Lưu trữ định nghĩa thuộc tính",
      ARCHIVE_DESCRIPTION:
        "Định nghĩa sẽ ngừng nhận dữ liệu mới nhưng lịch sử revision và audit vẫn được giữ nguyên.",
      CURRENT_REVISION: "Đang có hiệu lực",
      REVISION_LABEL: (revision: number) => `Bản sửa đổi ${revision}`,
      CANDIDATE_REVISION: "Revision ứng viên",
      ARCHIVED_STATE: "Sau khi lưu trữ",
      VERSION_CHANGE: "Phiên bản",
      AFFECTED_VALUES: "Giá trị nguồn",
      RETAINED_TARGET_VALUES: "Giá trị đích cũ",
      MIGRATION: "Di chuyển dữ liệu",
      MIGRATION_REQUIRED: "Cần chạy migration",
      MIGRATION_NOT_REQUIRED: "Không cần",
      MIGRATION_NOTICE_TITLE: "Hệ thống sẽ chạy migration có checkpoint",
      MIGRATION_NOTICE_DESCRIPTION:
        "Giá trị được chuyển theo lô và kiểm tra drift trước khi đổi con trỏ active. Đọc hồ sơ vẫn dùng revision cũ cho tới khi toàn bộ thao tác hoàn tất.",
      LIFECYCLE_REASON_PLACEHOLDER:
        "Nêu rõ lý do kích hoạt hoặc lưu trữ để người duyệt có đủ ngữ cảnh...",
      REVIEW_ACTION: "Tạo bản kiểm tra",
      REVIEWING: "Đang kiểm tra...",
      ACTIVATE_APPLY: "Xác nhận kích hoạt",
      ARCHIVE_APPLY: "Xác nhận lưu trữ",
      APPLYING: "Đang áp dụng...",
      REVIEW_ERROR: "Chưa thể tạo bản kiểm tra. Nội dung đang soạn vẫn được giữ lại.",
      APPLY_ERROR: "Chưa thể áp dụng thay đổi. Có thể thử lại an toàn với cùng mã lệnh.",
      CONFIRMATION_EXPIRED:
        "Bản kiểm tra đã hết hạn. Lý do được giữ lại để bạn tạo bản kiểm tra mới.",
      REAUTH_TITLE: "Xác minh lại danh tính",
      REAUTH_DESCRIPTION:
        "Thao tác thay đổi schema cần một lần xác thực gần đây từ đúng phiên đăng nhập này.",
      REAUTH_REQUIRED: "Hãy nhập mật khẩu hiện tại để xác minh lại trước khi áp dụng.",
      ACTIVATE_SUCCESS: "Revision thuộc tính đã được kích hoạt.",
      MIGRATION_SCHEDULED_SUCCESS:
        "Migration thuộc tính đã được xếp lịch. Revision hiện tại vẫn phục vụ hồ sơ cho tới khi operation hoàn tất.",
      ARCHIVE_SUCCESS: "Định nghĩa thuộc tính đã được lưu trữ.",
      TYPE_STRING: "Văn bản",
      TYPE_NUMBER: "Số",
      TYPE_BOOLEAN: "Đúng/Sai",
      TYPE_DATE: "Ngày",
    },

    // Effective-dated user reward profiles
    USER_TRAITS: {
      TITLE: "Hồ Sơ Phần Thưởng",
      SUBTITLE:
        "Xem và công bố một phiên bản gán căn cốt, thiên phú bất biến cho người dùng.",
      STEP_USER: "1. Chọn người dùng",
      STEP_USER_DESCRIPTION:
        "Tìm tài khoản cần kiểm tra hoặc điều chỉnh hồ sơ phần thưởng.",
      STEP_SELECTION: "2. Soạn cấu hình mới",
      STEP_SELECTION_DESCRIPTION:
        "Chọn đúng một căn cốt và ba thiên phú. Mỗi hiệu ứng được ghim vào revision hiện tại.",
      STEP_PREVIEW: "3. Kiểm tra và công bố",
      STEP_PREVIEW_DESCRIPTION:
        "Preview khóa revision và checksum trước khi tạo một phiên bản có hiệu lực mới.",
      CURRENT_TITLE: "Hồ sơ đang có hiệu lực",
      CURRENT_EMPTY: "Hãy chọn người dùng để tải hồ sơ hiện tại.",
      CURRENT_LOADING: "Đang tải hồ sơ phần thưởng...",
      CURRENT_LOAD_ERROR: "Không thể tải hồ sơ phần thưởng của người dùng này.",
      CATALOG_LOADING: "Đang tải danh mục căn cốt và thiên phú...",
      CATALOG_LOAD_ERROR: "Không thể tải danh mục thiên mệnh.",
      ROOT_LABEL: "Căn cốt · chọn 1",
      TALENT_LABEL: "Thiên phú · chọn 3",
      TALENT_PROGRESS: (selected: number) => `Đã chọn ${selected}/3 thiên phú`,
      PREVIEW_ACTION: "Tạo Preview",
      PREVIEWING: "Đang kiểm tra...",
      PREVIEW_TITLE: "Phiên bản sẽ công bố",
      PREVIEW_EMPTY:
        "Chọn đủ một căn cốt và ba thiên phú để tạo preview có kiểm chứng.",
      PREVIEW_REVISION: (current: number, next: number) =>
        `Revision ${current} → ${next}`,
      PREVIEW_CHECKSUM: "Checksum preview",
      REASON_LABEL: "Lý do điều chỉnh",
      REASON_PLACEHOLDER:
        "Mô tả rõ lý do thay đổi để ghi vào audit log...",
      REASON_HELP: "Từ 3 đến 1024 ký tự; không ghi dữ liệu nhạy cảm.",
      APPLY_ACTION: "Công Bố Hồ Sơ",
      APPLYING: "Đang công bố...",
      APPLY_SUCCESS: "Đã công bố hồ sơ phần thưởng mới.",
      APPLY_ERROR: "Chưa thể công bố hồ sơ phần thưởng.",
      CONFLICT:
        "Hồ sơ hoặc danh mục vừa thay đổi. Dữ liệu mới nhất đang được tải lại để preview lại.",
      READ_ONLY:
        "Bạn có quyền xem nhưng chưa có quyền công bố hồ sơ phần thưởng.",
      SELECTION_INCOMPLETE: "Cần chọn đúng một căn cốt và ba thiên phú.",
      SELECTED: "Đã chọn",
      EFFECT_UNAVAILABLE: "Thiên mệnh chưa có effect revision hợp lệ.",
      RETRY: "Tải Lại",
    },

    // User element EXP
    USER_ELEMENT_EXPS: {
      TITLE: "EXP Ngũ Hành Người Dùng",
      SUBTITLE: "Quản lý EXP ngũ hành của từng người dùng.",
      CREATE: "Tạo EXP Ngũ Hành",
      SEARCH_PLACEHOLDER: "Lọc theo UUID người dùng...",
      EMPTY: "Chưa có dữ liệu EXP ngũ hành",
      DIALOG_CREATE_TITLE: "Tạo EXP Ngũ Hành",
      DIALOG_CREATE_DESC: "Chọn người dùng và ngũ hành để khởi tạo EXP.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa EXP Ngũ Hành",
      DIALOG_EDIT_DESC: "Cập nhật EXP hiện tại của ngũ hành.",
      COL_USER_ID: "ID người dùng",
      COL_ELEMENT_ID: "ID ngũ hành",
      COL_EXP: "EXP",
    },

    // User statistics
    USER_STATS: {
      TITLE: "Chỉ Số Người Dùng",
      SUBTITLE: "Quản lý EXP tổng và rating của người dùng.",
      CREATE: "Tạo Chỉ Số",
      SEARCH_PLACEHOLDER: "Lọc theo UUID người dùng...",
      EMPTY: "Chưa có chỉ số người dùng",
      DIALOG_CREATE_TITLE: "Tạo Chỉ Số Người Dùng",
      DIALOG_CREATE_DESC: "Chọn người dùng để khởi tạo EXP tổng và rating.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Chỉ Số Người Dùng",
      DIALOG_EDIT_DESC: "Cập nhật EXP tổng và rating hiện tại.",
      COL_USER_ID: "ID người dùng",
      COL_TOTAL_EXP: "Tổng EXP",
      COL_RATING: "Rating",
    },

    // Ranks
    RANKS: {
      TITLE: "Danh Hiệu",
      SUBTITLE: "Quản lý các danh hiệu xếp hạng",
      CREATE: "Tạo Danh Hiệu",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có danh hiệu nào",
      DIALOG_CREATE_TITLE: "Tạo Danh Hiệu Mới",
      DIALOG_CREATE_DESC: "Thêm danh hiệu mới vào hệ thống.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Danh Hiệu",
      DIALOG_EDIT_DESC: "Cập nhật thông tin danh hiệu.",
      COL_NAME: "Tên",
      COL_MIN_RATING: "Rating tối thiểu",
      COL_DESC: "Mô tả",
    },

    // Levels
    LEVELS: {
      TITLE: "Cảnh Giới",
      SUBTITLE: "Quản lý các cảnh giới tu luyện",
      CREATE: "Tạo Cảnh Giới",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có cảnh giới nào",
      DIALOG_CREATE_TITLE: "Tạo Cảnh Giới Mới",
      DIALOG_CREATE_DESC: "Thêm cảnh giới mới vào hệ thống.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Cảnh Giới",
      DIALOG_EDIT_DESC: "Cập nhật thông tin cảnh giới.",
      COL_NAME: "Tên",
      COL_MIN_EXP: "EXP tối thiểu",
      COL_DESC: "Mô tả",
    },

    // Elements
    ELEMENTS: {
      TITLE: "Ngũ Hành",
      SUBTITLE: "Quản lý các hệ ngũ hành linh căn",
      CREATE: "Tạo Ngũ Hành",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có ngũ hành nào",
      DIALOG_CREATE_TITLE: "Tạo Ngũ Hành Mới",
      DIALOG_CREATE_DESC: "Thêm ngũ hành mới vào hệ thống.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Ngũ Hành",
      DIALOG_EDIT_DESC: "Cập nhật thông tin ngũ hành.",
      COL_NAME: "Tên",
      COL_CODE: "Mã code",
      COL_DESC: "Mô tả",
    },

    // Difficulties
    DIFFICULTIES: {
      TITLE: "Độ Khó",
      SUBTITLE: "Quản lý các mức độ khó của bài tập",
      CREATE: "Tạo Độ Khó",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có độ khó nào",
      DIALOG_CREATE_TITLE: "Tạo Độ Khó Mới",
      DIALOG_CREATE_DESC: "Thêm mức độ khó mới vào hệ thống.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Độ Khó",
      DIALOG_EDIT_DESC: "Cập nhật thông tin độ khó.",
      COL_NAME: "Tên",
      COL_LEVEL: "Cấp độ",
      COL_EXP: "EXP thưởng",
      COL_DESC: "Mô tả",
    },

    // Rarities
    RARITIES: {
      TITLE: "Phẩm Chất",
      SUBTITLE: "Quản lý các bậc phẩm chất vật phẩm",
      CREATE: "Tạo Phẩm Chất",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có phẩm chất nào",
      DIALOG_CREATE_TITLE: "Tạo Phẩm Chất Mới",
      DIALOG_CREATE_DESC: "Thêm phẩm chất mới vào hệ thống.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Phẩm Chất",
      DIALOG_EDIT_DESC: "Cập nhật thông tin phẩm chất.",
      COL_NAME: "Tên",
      COL_CODE: "Mã code",
      COL_WEIGHT: "Trọng số",
      COL_DESC: "Mô tả",
    },

    // Traits
    TRAITS: {
      TITLE: "Căn Cốt & Thiên Phú",
      SUBTITLE: "Quản lý các thiên mệnh của người tu luyện",
      CREATE: "Tạo Thiên Mệnh",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có thiên mệnh nào",
      DIALOG_CREATE_TITLE: "Tạo Thiên Mệnh Mới",
      DIALOG_CREATE_DESC: "Thêm căn cốt hoặc thiên phú mới vào hệ thống.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Thiên Mệnh",
      DIALOG_EDIT_DESC: "Cập nhật thông tin thiên mệnh.",
      COL_NAME: "Tên",
      COL_TYPE: "Loại",
      COL_RARITY: "Phẩm chất",
      COL_DESC: "Mô tả",
      COL_EFFECT: "Hiệu ứng EXP",
      COL_REVISION: "Revision",
      COL_STATUS: "Trạng thái",
      ACTIVE: "Đang hoạt động",
      ARCHIVED: "Đã lưu trữ",
      CODE_LABEL: "Mã ổn định",
      CODE_PLACEHOLDER: "VD: keen_mind",
      DISPLAY_ORDER: "Thứ tự hiển thị",
      EFFECT_KIND: "Loại hiệu ứng",
      EFFECT_MULTIPLIER: "Điều chỉnh tỷ lệ EXP",
      EFFECT_BONUS: "Cộng EXP cố định",
      EFFECT_SCOPE: "Phạm vi áp dụng",
      EFFECT_SCOPE_ALL: "Tất cả bài giải",
      EFFECT_SCOPE_ELEMENT: "Theo ngũ hành",
      EFFECT_VALUE_BPS: "Mức thay đổi (basis points)",
      EFFECT_VALUE_BONUS: "EXP cộng thêm",
      EFFECT_ELEMENTS: "Mã ngũ hành",
      EFFECT_ELEMENTS_PLACEHOLDER: "fire, water",
      EFFECT_ELEMENTS_HELP:
        "Nhập mã ngũ hành, phân tách bằng dấu phẩy. Tối đa 16 mã.",
      REASON_LABEL: "Lý do thay đổi",
      REASON_PLACEHOLDER: "Mô tả lý do để ghi vào audit log...",
      ARCHIVE_ACTION: "Lưu Trữ Thiên Mệnh",
      ARCHIVE_TITLE: "Lưu trữ thiên mệnh này?",
      ARCHIVE_DESCRIPTION:
        "Thiên mệnh sẽ ngừng xuất hiện trong lựa chọn mới; các revision và hồ sơ cũ vẫn được giữ nguyên để tái hiện phần thưởng.",
      ARCHIVE_SUCCESS: "Đã lưu trữ thiên mệnh.",
      ARCHIVE_ERROR: "Chưa thể lưu trữ thiên mệnh.",
      FILTER_TYPE_ALL: "Tất cả loại",
      FILTER_RARITY_ALL: "Tất cả phẩm chất",
    },

    // Tags
    TAGS: {
      TITLE: "Công Pháp (Tags)",
      SUBTITLE: "Quản lý các thẻ phân loại bài tập",
      CREATE: "Tạo Tag",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có tag nào",
      DIALOG_CREATE_TITLE: "Tạo Tag Mới",
      DIALOG_CREATE_DESC: "Thêm tag mới vào hệ thống.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Tag",
      DIALOG_EDIT_DESC: "Cập nhật thông tin tag.",
      COL_NAME: "Tên",
      COL_ELEMENTS: "Ngũ hành",
      COL_DESC: "Mô tả",
      FILTER_ELEMENT_ALL: "Tất cả ngũ hành",
    },

    // Problems (list page only — create/edit navigate to separate pages)
    PROBLEMS: {
      TITLE: "Bài Tập",
      SUBTITLE: "Quản lý bài tập và test cases",
      CREATE: "Tạo Bài Tập",
      SEARCH_PLACEHOLDER: "Tìm theo tiêu đề...",
      EMPTY: "Chưa có bài tập nào",
      FILTER_DIFFICULTY_ALL: "Tất cả độ khó",
      FILTER_DIFFICULTY_LABEL: "Lọc bài tập theo độ khó",
      FILTER_DIFFICULTY_LOADING: "Đang tải độ khó...",
      FILTER_DIFFICULTY_ERROR: "Không thể tải bộ lọc độ khó.",
      FILTER_DIFFICULTY_RETRY: "Tải lại bộ lọc",
      COL_TITLE: "Tiêu đề",
      COL_TAGS: "Tags",
      COL_DIFFICULTY: "Độ khó",
      COL_STATUS: "Trạng thái",
      COL_LIMITS: "Giới hạn",
    },

    // Contests
    CONTESTS: {
      TITLE: "Đại Hội Tỷ Thí",
      SUBTITLE: "Quản lý các cuộc thi lập trình",
      CREATE: "Tạo Đại Hội",
      SEARCH_PLACEHOLDER: "Tìm theo tiêu đề...",
      EMPTY: "Chưa có cuộc thi nào",
      COL_TITLE: "Tiêu đề",
      COL_STATUS: "Trạng thái",
      COL_START: "Bắt đầu",
      COL_END: "Kết thúc",
      COL_PARTICIPANTS: "Tham gia",
      COL_RATING: "Rating",
      DIALOG_CREATE_TITLE: "Tạo Đại Hội Mới",
      DIALOG_CREATE_DESC: "Tạo cuộc thi lập trình mới.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Đại Hội",
      DIALOG_EDIT_DESC: "Cập nhật thông tin cuộc thi.",
      FORM_TITLE: "Tiêu đề",
      FORM_TITLE_PLACEHOLDER: "VD: Đại Hội Mùa Xuân 2026",
      FORM_DESCRIPTION: "Mô tả",
      FORM_DESCRIPTION_PLACEHOLDER: "Mô tả cuộc thi...",
      FORM_START_TIME: "Thời gian bắt đầu",
      FORM_END_TIME: "Thời gian kết thúc",
      FORM_MAX_PARTICIPANTS: "Giới hạn người tham gia",
      FORM_CAPACITY_UNLIMITED: "Để trống nếu không giới hạn",
      FORM_FREEZE_TIME: "Thời điểm đóng băng bảng xếp hạng",
      FORM_RATED: "Tính rating",
      FORM_RATED_HINT:
        "Áp dụng expected-rank-v1 sau khi toàn bộ chấm bài chính thức đã ổn định.",
      FORM_SCORING_POLICY: "Cách tính điểm",
      SCORING_ICPC: "ICPC — số bài giải và thời gian phạt",
      SERVER_MANAGED: "Hệ thống quản lý",
      SECTION_OVERVIEW: "Thông tin đại hội",
      SECTION_SCHEDULE: "Lịch thi đấu",
      SECTION_REGISTRATION: "Chính sách tham gia",
      SECTION_PROBLEMS: "Bộ đề thi",
      FORM_REGISTRATION_MODE: "Hình thức tham gia",
      REGISTRATION_REGISTERED: "Đăng ký trước",
      REGISTRATION_OPEN: "Tham gia mở",
      REGISTRATION_REGISTERED_HINT:
        "Thí sinh phải đăng ký trong thời gian quy định trước khi đại hội bắt đầu.",
      REGISTRATION_OPEN_HINT:
        "Thí sinh được ghi nhận tự động khi gửi bài chính thức đầu tiên.",
      FORM_REGISTRATION_OPENS: "Mở đăng ký",
      FORM_REGISTRATION_CLOSES: "Đóng đăng ký",
      ERROR_TIME_WINDOW:
        "Thời gian kết thúc phải nằm sau thời gian bắt đầu.",
      ERROR_PROBLEM_ALIAS:
        "Ký hiệu bài phải duy nhất, bắt đầu bằng chữ in hoa và dài tối đa 16 ký tự.",
      PROBLEM_SELECTION_HINT:
        "Tìm trong kho bài đã xuất bản, sắp thứ tự và kiểm soát nội dung hiển thị trước giờ thi.",
      PROBLEM_SELECTED_COUNT: (count: number) => `${count}/500 bài`,
      PROBLEM_SEARCH_PLACEHOLDER: "Tìm theo tên hoặc slug...",
      PROBLEM_SEARCH_LABEL: "Tìm bài đã xuất bản để thêm vào đại hội",
      PROBLEM_SEARCHING: "Đang tìm trong kho bài...",
      PROBLEM_SEARCH_ERROR:
        "Chưa thể tải kho bài. Hãy thử lại sau ít giây.",
      PROBLEM_SEARCH_EMPTY: "Không còn bài phù hợp để thêm.",
      PROBLEM_SELECTED_EMPTY: "Bộ đề đang để trống",
      PROBLEM_SELECTED_EMPTY_HINT:
        "Bạn có thể lưu bản nháp trước; cần ít nhất một bài hợp lệ trước khi xuất bản.",
      PROBLEM_MOVE_UP: "Đưa bài lên trên",
      PROBLEM_MOVE_DOWN: "Đưa bài xuống dưới",
      PROBLEM_REMOVE: "Bỏ bài khỏi đại hội",
      PROBLEM_ALIAS: "Ký hiệu",
      PROBLEM_VISIBLE_BEFORE: "Hiện trước giờ thi",
      DRAFT_VERSION_HINT:
        "Mỗi lần lưu tạo một phiên bản mới để ngăn ghi đè thay đổi đồng thời.",
      DRAFT_REASON_LABEL: "Lý do thay đổi",
      DRAFT_REASON_PLACEHOLDER:
        "Mô tả ngắn mục đích của lần tạo hoặc chỉnh sửa bản nháp này.",
      DRAFT_REASON_HINT: (current: number, maximum: number) =>
        `${current}/${maximum} byte UTF-8. Lý do được lưu cùng audit và sự kiện bền vững.`,
      DELETE_REASON_LABEL: "Lý do xóa bản nháp",
      DELETE_REASON_PLACEHOLDER:
        "Nêu lý do bản nháp này không còn cần thiết.",
      DELETE_REASON_HINT: (current: number, maximum: number) =>
        `${current}/${maximum} byte UTF-8. Thao tác xóa sẽ được kiểm toán.`,
      FILTER_STATUS_ALL: "Tất cả trạng thái",
      PUBLISH: "Xuất Bản",
      PUBLISH_ACTION: "Xuất bản đại hội",
      PUBLISH_CONFIRM_TITLE: "Xuất Bản Đại Hội?",
      PUBLISH_CONFIRM_DESCRIPTION: (title: string) =>
        `“${title}” sẽ chuyển sang trạng thái sắp diễn ra và không thể chỉnh sửa nội dung nữa.`,
      PUBLISH_REASON_LABEL: "Lý do xuất bản",
      PUBLISH_REASON_PLACEHOLDER:
        "Mô tả ngắn mục đích và phạm vi của lần xuất bản này.",
      PUBLISH_REASON_HINT: (current: number, maximum: number) =>
        `${current}/${maximum} byte UTF-8. Lý do sẽ được lưu trong nhật ký kiểm toán.`,
      PUBLISH_SUCCESS: "Đại hội đã được xuất bản.",
      PUBLISH_ERROR: "Không thể xuất bản đại hội",
      END: "Kết Thúc",
      END_ACTION: "Kết thúc đại hội ngay",
      END_CONFIRM_TITLE: "Kết Thúc Đại Hội Sớm?",
      END_CONFIRM_DESCRIPTION: (title: string) =>
        `“${title}” sẽ ngừng nhận bài chính thức ngay sau khi lệnh được xác nhận.`,
      END_SUCCESS: "Đại hội đã kết thúc.",
      END_ERROR: "Không thể kết thúc đại hội",
      CANCEL: "Hủy Đại Hội",
      CANCEL_ACTION: "Hủy đại hội",
      CANCEL_CONFIRM_TITLE: "Hủy Đại Hội?",
      CANCEL_CONFIRM_DESCRIPTION: (title: string) =>
        `“${title}” sẽ chuyển sang trạng thái đã hủy và không thể mở lại.`,
      CANCEL_SUCCESS: "Đại hội đã được hủy.",
      CANCEL_ERROR: "Không thể hủy đại hội",
      LIFECYCLE_REASON_LABEL: "Lý do vận hành",
      LIFECYCLE_REASON_PLACEHOLDER:
        "Nêu rõ nguyên nhân và phạm vi ảnh hưởng của thay đổi lifecycle này.",
      LIFECYCLE_REASON_HINT: (current: number, maximum: number) =>
        `${current}/${maximum} byte UTF-8. Lý do, người thao tác và CID được lưu trong audit bền vững.`,
      RERATE: "Tính Lại",
      RERATE_ACTION: "Tính lại rating từ đại hội này",
      RERATE_CONFIRM_TITLE: "Tính Lại Rating Lịch Sử?",
      RERATE_CONFIRM_DESCRIPTION: (title: string) =>
        `Hệ thống sẽ tính lại “${title}” và toàn bộ đại hội có rating diễn ra sau đó.`,
      RERATE_SAFETY_NOTE:
        "Một projection ẩn sẽ được dựng và kiểm tra đầy đủ. Rating hiện tại vẫn phục vụ bình thường cho đến khi hệ thống đổi phiên bản nguyên tử.",
      RERATE_REASON_LABEL: "Lý do tính lại",
      RERATE_REASON_PLACEHOLDER:
        "Nêu sự cố chấm bài, thay đổi kết quả hoặc lý do vận hành cần tính lại...",
      RERATE_REASON_HINT: (current: number, maximum: number) =>
        `${current}/${maximum} byte UTF-8. Tối thiểu 3 ký tự; lý do được lưu trong audit và immutable rating run.`,
      RERATE_START: "Bắt Đầu Tính Lại",
      RERATE_SUCCESS:
        "Đã đưa tác vụ tính lại rating vào control plane.",
      RERATE_ERROR: "Không thể bắt đầu tính lại rating",
      RATING_DISABLED: "Không tính",
      RATING_ENABLED: "Đã bật",
      RATING_PENDING: "Đang chờ",
      RATING_APPLIED: "Đã tính",
      OVERVIEW_TITLE: "Trung Tâm Đại Hội",
      OVERVIEW_SUBTITLE:
        "Theo dõi lifecycle, projection bảng xếp hạng chính thức và rating đang được phục vụ.",
      OVERVIEW_BACK: "Quay lại danh sách đại hội",
      OVERVIEW_PUBLIC: "Mở trang thí sinh",
      OVERVIEW_COMMUNICATIONS: "Thông báo & làm rõ",
      OVERVIEW_LOAD_ERROR: "Không thể tải trung tâm đại hội",
      OVERVIEW_LOAD_ERROR_DESCRIPTION:
        "Không có dữ liệu giả thay thế. Hãy thử đồng bộ lại projection thật từ máy chủ.",
      OVERVIEW_RETRY: "Tải lại",
      OVERVIEW_PARTICIPANTS: "Thí sinh",
      OVERVIEW_PROBLEMS: "Bài thi",
      OVERVIEW_STANDINGS_VERSION: "Phiên bản BXH",
      OVERVIEW_RATING_STATE: "Trạng thái rating",
      OVERVIEW_SCHEDULE: "Lịch đại hội",
      OVERVIEW_START: "Bắt đầu",
      OVERVIEW_END: "Kết thúc",
      OVERVIEW_FREEZE: "Đóng băng công khai",
      OVERVIEW_NO_FREEZE: "Không đóng băng",
      OVERVIEW_ALGORITHM: "Thuật toán rating",
      OVERVIEW_OFFICIAL_TAB: "Bảng Xếp Hạng Chính Thức",
      OVERVIEW_RATING_TAB: "Biến Động Rating",
      OVERVIEW_OFFICIAL_DESCRIPTION:
        "Projection đầy đủ dành cho ban tổ chức; không áp dụng che kết quả sau thời điểm freeze.",
      OVERVIEW_RATING_DESCRIPTION:
        "Immutable run hiện được active run-chain lựa chọn. Rerating chỉ xuất hiện sau atomic switch.",
      OVERVIEW_LIVE: "Đồng bộ thời gian thực",
    },

    MATERIAL_CATEGORIES: {
      TITLE: "Danh Mục Tàng Kinh Các",
      SUBTITLE: "Quản lý danh mục bài viết học tập",
      CREATE: "Tạo Danh Mục",
      SEARCH_PLACEHOLDER: "Tìm theo tên...",
      EMPTY: "Chưa có danh mục nào",
      COL_NAME: "Tên",
      COL_DESC: "Mô tả",
      COL_ARTICLES: "Bài viết",
      DIALOG_CREATE_TITLE: "Tạo Danh Mục Mới",
      DIALOG_CREATE_DESC: "Thêm danh mục mới cho Tàng Kinh Các.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Danh Mục",
      DIALOG_EDIT_DESC: "Cập nhật thông tin danh mục.",
      FORM_NAME: "Tên danh mục",
      FORM_NAME_PLACEHOLDER: "VD: Cấu Trúc Dữ Liệu",
      FORM_DESCRIPTION: "Mô tả",
      FORM_DESCRIPTION_PLACEHOLDER: "Mô tả ngắn về danh mục...",
    },

    MATERIALS: {
      TITLE: "Tàng Kinh Các",
      SUBTITLE: "Quản lý bài viết học tập",
      CREATE: "Tạo Bài Viết",
      SEARCH_PLACEHOLDER: "Tìm theo tiêu đề...",
      EMPTY: "Chưa có bài viết nào",
	  EDITOR_LOAD_ERROR:
		"Không thể tải đầy đủ bản nháp để chỉnh sửa. Hãy thử lại.",
	  MUTATION_CONFLICT:
		"Tài liệu hoặc đường dẫn vừa thay đổi trên máy chủ. Danh sách đã được tải lại; hãy mở lại tài liệu trước khi thử tiếp.",
      COL_TITLE: "Tiêu đề",
      COL_CATEGORY: "Danh mục",
      COL_DIFFICULTY: "Độ khó",
      COL_STATUS: "Trạng thái",
      COL_VIEWS: "Lượt xem",
      COL_READ_TIME: "Thời gian đọc",
      DIALOG_CREATE_TITLE: "Tạo Bài Viết Mới",
      DIALOG_CREATE_DESC: "Viết bài học tập mới.",
      DIALOG_EDIT_TITLE: "Chỉnh Sửa Bài Viết",
      DIALOG_EDIT_DESC: "Cập nhật nội dung bài viết.",
      FORM_TITLE: "Tiêu đề",
      FORM_TITLE_PLACEHOLDER: "VD: Giới thiệu Stack và Queue",
      FORM_SLUG: "Đường dẫn",
      FORM_SLUG_PLACEHOLDER: "VD: gioi-thieu-stack-va-queue",
      FORM_DESCRIPTION: "Mô tả",
      FORM_DESCRIPTION_PLACEHOLDER: "Mô tả ngắn về bài viết...",
      FORM_CONTENT: "Nội dung (Markdown)",
      FORM_CONTENT_PLACEHOLDER: "Viết nội dung bài viết ở đây...",
      FORM_CATEGORY: "Danh mục",
      FORM_DIFFICULTY: "Độ khó",
      FORM_TAGS: "Nhãn",
      FORM_STATUS: "Trạng thái",
      FORM_VISIBILITY: "Hiển thị",
      FORM_CHANGE_SUMMARY: "Tóm tắt thay đổi",
      FORM_CHANGE_SUMMARY_PLACEHOLDER:
        "Mô tả nội dung và lý do của phiên bản này...",
      FORM_CHANGE_SUMMARY_HINT:
        "Tóm tắt được lưu cùng phiên bản bất biến và nhật ký kiểm toán.",
      STATUS_DRAFT: "Nháp",
      STATUS_IN_REVIEW: "Chờ duyệt",
      STATUS_PUBLISHED: "Đã xuất bản",
      STATUS_ARCHIVED: "Đã lưu trữ",
      VISIBILITY_PUBLIC: "Công khai",
      VISIBILITY_AUTHENTICATED: "Người dùng đã đăng nhập",
      FILTER_CATEGORY_ALL: "Tất cả danh mục",
      FILTER_DIFFICULTY_ALL: "Tất cả độ khó",
      FILTER_STATUS_ALL: "Tất cả trạng thái",
      PUBLISH: "Xuất bản phiên bản hiện tại",
		SUBMIT_REVIEW: "Gửi duyệt tài liệu",
		RETURN_TO_DRAFT: "Trả tài liệu về bản nháp",
      ARCHIVE: "Lưu trữ tài liệu",
      ROLLBACK: "Khôi phục phiên bản",
      RENAME: "Đổi đường dẫn tài liệu",
      ROLLBACK_REVISION: "Phiên bản cần khôi phục",
      ROLLBACK_REVISION_PLACEHOLDER: "Chọn một phiên bản đã lưu",
      ROLLBACK_REVISION_LOADING: "Đang tải lịch sử phiên bản...",
      ROLLBACK_REVISION_LOAD_ERROR:
        "Không thể tải lịch sử phiên bản. Hãy thử lại.",
      ROLLBACK_REVISION_LOAD_MORE: "Tải thêm phiên bản",
      ROLLBACK_REVISION_EMPTY: "Chưa có phiên bản cũ để khôi phục.",
      ROLLBACK_REVISION_NUMBER: (revision: number) =>
        `Phiên bản ${revision}`,
	  ROLLBACK_REVISION_CURRENT: "Đang là bản nháp hoặc bản xuất bản hiện tại",
	  ROLLBACK_REVISION_NOT_PRIOR: "Không cũ hơn bản đang xuất bản",
      ROLLBACK_REVISION_CREATED_AT: (value: string) => `Tạo lúc ${value}`,
      LIFECYCLE_REASON: "Lý do thao tác",
      LIFECYCLE_REASON_PLACEHOLDER:
        "Nêu rõ lý do để lưu trong nhật ký kiểm toán...",
      LIFECYCLE_SUCCESS: "Trạng thái tài liệu đã được cập nhật.",
      LIFECYCLE_REPLAY_SUCCESS:
        "Lệnh đã được xử lý trước đó; hệ thống vừa khôi phục đúng biên nhận đã cam kết.",
      LIFECYCLE_ERROR:
        "Không thể cập nhật trạng thái tài liệu. Dữ liệu hiện tại không thay đổi.",
      LIFECYCLE_REVIEW_INVALID:
        "Bản xem trước đã hết hiệu lực hoặc tài liệu vừa thay đổi. Nội dung bạn nhập vẫn được giữ lại; hãy xem trước lại.",
      LIFECYCLE_RECENT_AUTH_REQUIRED:
        "Phiên xác thực không còn đủ mới cho thao tác nhạy cảm. Hãy đăng nhập lại, sau đó tạo bản xem trước mới.",
      LIFECYCLE_REVIEW_DESCRIPTION:
        "Đối chiếu thay đổi do máy chủ xác minh trước khi áp dụng lệnh.",
      LIFECYCLE_CREATE_REVIEW: "Tạo bản xem trước",
      LIFECYCLE_APPLY_REVIEWED: "Xác nhận và áp dụng",
      LIFECYCLE_REVIEW_TITLE: "Bản xem trước đã được xác minh",
      LIFECYCLE_REVIEW_HELP:
        "Bản xem trước này chỉ hợp lệ cho đúng tài liệu, phiên bản, lý do và phiên đăng nhập hiện tại.",
      LIFECYCLE_EXPIRES_AT: (value: string) => `Hết hạn ${value}`,
      LIFECYCLE_BEFORE: "Trước thay đổi",
      LIFECYCLE_AFTER: "Sau thay đổi",
      LIFECYCLE_CHANGE_DIRECTION: "Thay đổi từ trạng thái trước sang trạng thái sau",
      LIFECYCLE_SERVER_VERIFIED:
        "Phiên bản nội dung và bản dựng hiển thị đích đã được máy chủ kiểm tra. Hệ thống sẽ kiểm tra lại trong giao dịch dữ liệu khi áp dụng.",
      LIFECYCLE_FIELD_STATUS: "Trạng thái",
      LIFECYCLE_FIELD_VERSION: "Phiên bản dữ liệu",
      LIFECYCLE_FIELD_REVISION: "Phiên bản nội dung",
      LIFECYCLE_FIELD_TITLE: "Tiêu đề",
      LIFECYCLE_FIELD_VISIBILITY: "Phạm vi hiển thị",
      LIFECYCLE_FIELD_TAGS: "Nhãn đã chuẩn hoá",
      LIFECYCLE_FIELD_ARTIFACT: "Bản dựng hiển thị",
      LIFECYCLE_FIELD_PIPELINE: "Bộ dựng / làm sạch",
      LIFECYCLE_FIELD_CHECKSUM: "Mã kiểm toàn vẹn",
      LIFECYCLE_TAG_COUNT: (count: number) => `${count} nhãn`,
      LIFECYCLE_TITLE: {
        publish: "Xuất bản tài liệu?",
		submit_review: "Gửi tài liệu để duyệt?",
		return_to_draft: "Trả tài liệu về bản nháp?",
        archive: "Lưu trữ tài liệu?",
        rollback: "Khôi phục phiên bản?",
        rename: "Đổi đường dẫn tài liệu?",
      },
      LIFECYCLE_DESCRIPTION: {
        publish:
          "Phiên bản nháp và render artifact hiện tại sẽ được công bố nguyên vẹn.",
		submit_review:
		  "Bản nháp hiện tại sẽ được khoá trạng thái để người duyệt kiểm tra; nội dung không bị xuất bản.",
		return_to_draft:
		  "Tài liệu sẽ được trả về bản nháp kèm lý do để tác giả chỉnh sửa; lịch sử revision vẫn được giữ nguyên.",
        archive:
          "Tài liệu sẽ biến mất khỏi danh sách công khai nhưng lịch sử vẫn được giữ lại.",
        rollback:
          "Phiên bản bất biến đã chọn sẽ được xuất bản lại; lịch sử hiện tại không bị ghi đè.",
        rename:
          "Đường dẫn hiện tại sẽ được giữ làm liên kết chuyển hướng vĩnh viễn.",
      },
      LIFECYCLE_CONFIRM: {
        publish: "Xuất bản",
		submit_review: "Gửi duyệt",
		return_to_draft: "Trả về nháp",
        archive: "Lưu trữ",
        rollback: "Khôi phục",
        rename: "Đổi đường dẫn",
      },
    },

    // Shared action toast messages
    CREATE_SUCCESS: "Tạo thành công",
    UPDATE_SUCCESS: "Cập nhật thành công",
    DELETE_SUCCESS: "Xoá thành công",
    LOAD_ERROR: "Không thể tải dữ liệu",
    SAVE_ERROR: "Thao tác thất bại",
    DELETE_ERROR: "Không thể xoá",
} as const;
