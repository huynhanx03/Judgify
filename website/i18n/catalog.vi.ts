/**
 * Canonical Vietnamese product-authored copy.
 * Domain, user, contest, problem, material, and administrator-authored data
 * remains in API DTOs and must not be copied into this catalog.
 */


export const vi = {
  // App-level
  APP_NAME: "Judgify",
  APP_DESCRIPTION: "Thánh Địa Luyện Code",
  APP_FULL_TITLE: "Judgify - Thánh Địa Luyện Code",

  ROUTE_ERROR: {
    EYEBROW: "Kết nối gián đoạn",
    TITLE: "Trang này chưa thể hiển thị",
    DESCRIPTION:
      "Một lỗi ngoài dự kiến vừa xảy ra. Dữ liệu của bạn vẫn được giữ nguyên; hãy thử tải lại trang.",
    RETRY: "Thử Lại",
    BACK_HOME: "Về Luyện Tập",
  },

  NOT_FOUND: {
    EYEBROW: "404 · Lạc lối",
    TITLE: "Không tìm thấy đường dẫn này",
    DESCRIPTION:
      "Trang có thể đã được di chuyển, chưa được công bố hoặc địa chỉ bạn mở không hợp lệ.",
    BACK_HOME: "Về Luyện Tập",
  },

  META: {
    LOGIN_TITLE: "Đăng Nhập | Judgify",
    REGISTER_TITLE: "Khai Căn Trúc Cốt | Judgify",
    RESET_PASSWORD_TITLE: "Đặt Lại Mật Khẩu | Judgify",
    ACCEPT_INVITATION_TITLE: "Hoàn Tất Tài Khoản",
    ACCEPT_INVITATION_DESCRIPTION:
      "Thiết lập mật khẩu để kích hoạt tài khoản Judgify từ một lời mời bảo mật.",
    VERIFY_RECOVERY_CONTACT_TITLE: "Xác Minh Email Khôi Phục",
    OAUTH_CALLBACK_TITLE: "Hoàn Tất Xác Minh | Judgify",
    ABOUT_TITLE: "Tông Môn Bí Lục | Judgify",
		PUBLIC_PROFILE_TITLE: "Hồ Sơ Công Khai | Judgify",
		PUBLIC_PROFILE_DESCRIPTION:
			"Xem thông tin hồ sơ mà một thành viên Judgify chủ động chia sẻ công khai.",
  },

  // Navigation
  NAV: {
    ARENA: "Luyện Tập",
    MATERIALS: "Tàng Kinh Các",
    CONTEST: "Đại Hội Tỷ Thí",
    RANKING: "Bảng Phong Thần",
    PROFILE: "Hồ Sơ Tu Luyện",
    SETTINGS: "Cài Đặt",
    LOGOUT: "Đăng Xuất",
    LOGGING_OUT: "Đang đăng xuất",
    LOGOUT_FAILED: "Không thể đăng xuất an toàn. Vui lòng thử lại.",
    OPEN_SIDEBAR: "Mở menu quản trị",
    CLOSE_SIDEBAR: "Đóng menu quản trị",
    OPEN_MENU: "Mở điều hướng",
    SKIP_TO_CONTENT: "Chuyển đến nội dung chính",
    PRIMARY_LABEL: "Điều hướng chính",
    MOBILE_MENU_TITLE: "Điều hướng Judgify",
    MOBILE_MENU_DESCRIPTION: "Chọn khu vực bạn muốn khám phá.",
    ABOUT: "Tông Môn Bí Lục",
  },

  // Header User Dropdown
  HEADER: {
    AVATAR_FALLBACK: "ĐH",
    GUEST_NAME: "Đạo Hữu Vô Danh",
    GUEST_RANK: "Trúc Cơ Kỳ",
  },

  NOTIFICATION: {
    TITLE: "Thông Báo",
    EYEBROW: "Hộp thư của bạn",
    DESCRIPTION:
      "Theo dõi kết quả chấm, cuộc thi và những thay đổi quan trọng của tài khoản.",
    OPEN: "Mở thông báo",
    VIEW_ALL: "Xem tất cả thông báo",
    SETTINGS: "Tùy chỉnh thông báo",
    UNREAD_BADGE: (count: number) => `${count} thông báo chưa đọc`,
    UNREAD_SHORT: (count: number) => `${count} chưa đọc`,
    UNREAD_OVERFLOW: (maximum: number) => `Hơn ${maximum} thông báo chưa đọc`,
    MARK_ALL_READ: "Đánh dấu tất cả đã đọc",
    MARKING_ALL_READ: "Đang cập nhật...",
    MARK_READ: "Đánh dấu đã đọc",
    ARCHIVE: "Lưu trữ thông báo",
    OPEN_ACTION: "Mở nội dung liên quan",
    LOAD_MORE: "Xem thêm thông báo",
    LOADING_MORE: "Đang tải thêm...",
    LOADING: "Đang đồng bộ hộp thư...",
    EMPTY_TITLE: "Hộp thư đã gọn gàng",
    EMPTY_DESCRIPTION:
      "Thông báo mới về bài nộp, cuộc thi và tài khoản sẽ xuất hiện tại đây.",
    ERROR_TITLE: "Chưa thể tải thông báo",
    ERROR_DESCRIPTION:
      "Hộp thư vẫn được lưu an toàn. Hãy thử đồng bộ lại sau ít giây.",
    RETRY: "Đồng bộ lại",
    ACTION_ERROR: "Chưa thể cập nhật thông báo. Vui lòng thử lại.",
    PREFERENCE_ACTION_ERROR:
      "Chưa thể lưu tùy chọn. Thiết lập mới nhất đã được đồng bộ lại.",
    LIVE: "Đang cập nhật trực tiếp",
    RECONNECTING: "Đang kết nối lại; dữ liệu vẫn được đối soát qua máy chủ",
    UPDATED_AT: (value: string) => `Cập nhật ${value}`,
    NEW: "Mới",
    READ: "Đã đọc",
    CAMPAIGN: {
      READ: "Đọc bản tin",
      DIALOG_TITLE: "Bản Tin Judgify",
      DIALOG_DESCRIPTION:
        "Nội dung chính thức được tải từ revision bất biến đã phát cho tài khoản của bạn.",
      LOADING: "Đang tải nội dung bản tin...",
      ERROR_TITLE: "Chưa thể mở bản tin",
      ERROR_DESCRIPTION:
        "Nội dung có thể đã hết hạn hoặc tạm thời không khả dụng. Hãy thử lại sau ít giây.",
      REVISION: (id: string) => `Revision ${id}`,
    },
    PREFERENCES: {
      TITLE: "Tùy Chọn Thông Báo",
      DESCRIPTION:
        "Chọn nhóm nội dung bạn muốn nhận trong Judgify. Thông báo bảo mật và vận hành bắt buộc luôn được bật.",
      LOADING: "Đang tải tùy chọn...",
      ERROR_TITLE: "Chưa thể tải tùy chọn",
      ERROR_DESCRIPTION:
        "Thiết lập hiện tại chưa thay đổi. Hãy thử đồng bộ lại.",
      RETRY: "Tải lại tùy chọn",
      CHANNEL: "Trong ứng dụng",
      REQUIRED: "Bắt buộc khi đăng ký",
      REQUIRED_DESCRIPTION:
        "Nhóm này bảo vệ tài khoản hoặc tính toàn vẹn hệ thống nên không thể tắt.",
      ENABLE: (category: string) => `Bật thông báo ${category}`,
      DISABLE: (category: string) => `Tắt thông báo ${category}`,
    },
    CATEGORY: {
      AUTHORIZATION: {
        LABEL: "Quyền truy cập",
        DESCRIPTION: "Vai trò, quyền hạn và thay đổi phạm vi truy cập.",
      },
      CAMPAIGN: {
        LABEL: "Bản tin Judgify",
        DESCRIPTION: "Thông báo và cập nhật sản phẩm từ Judgify.",
      },
      CONTEST: {
        LABEL: "Cuộc thi",
        DESCRIPTION: "Đăng ký, lịch thi, công bố và kết quả cuộc thi.",
      },
      MATERIAL: {
        LABEL: "Tài liệu",
        DESCRIPTION: "Tài liệu mới trong những chủ đề bạn theo dõi.",
      },
      OPERATION: {
        LABEL: "Vận hành hệ thống",
        DESCRIPTION: "Sự cố có ảnh hưởng trực tiếp đến dữ liệu hoặc lượt chấm.",
      },
      SECURITY: {
        LABEL: "Bảo mật tài khoản",
        DESCRIPTION: "Phiên đăng nhập và những thay đổi nhạy cảm của tài khoản.",
      },
      SUBMISSION: {
        LABEL: "Bài nộp",
        DESCRIPTION: "Kết quả chấm và trạng thái xử lý bài nộp.",
      },
    },
    MESSAGE: {
      FALLBACK_TITLE: "Có cập nhật mới",
      FALLBACK_BODY: "Mở thông báo để xem thông tin mới nhất từ Judgify.",
      PARAMETER_FALLBACK: {
        EMPTY: "—",
        SUBMISSION: "Bài nộp",
        SUBMISSION_REFERENCE: "bài nộp này",
        VERDICT: "đã hoàn tất",
        CONTEST: "Cuộc thi",
        CONTEST_REFERENCE: "cuộc thi",
        REGISTRATION_STATUS: "đã cập nhật đăng ký",
        LIFECYCLE_STATUS: "đã cập nhật",
        RATING_DELTA: "0",
        DEVICE: "một thiết bị",
        MATERIAL: "Một tài liệu mới",
      },
      CAMPAIGN_TITLE: "Bản tin từ Judgify",
      CAMPAIGN_BODY: "Mở thông báo để xem nội dung bản tin mới nhất.",
      SUBMISSION_JUDGED_TITLE: "Bài nộp đã được chấm",
      SUBMISSION_JUDGED_BODY: (problem: string, verdict: string) =>
        `${problem}: kết quả ${verdict}.`,
      SUBMISSION_SYSTEM_ERROR_TITLE: "Lượt chấm cần được xử lý lại",
      SUBMISSION_SYSTEM_ERROR_BODY: (problem: string) =>
        `Hệ thống chưa thể hoàn tất lượt chấm cho ${problem}. Judgify sẽ tự động thử lại.`,
      CONTEST_REGISTRATION_TITLE: "Trạng thái đăng ký cuộc thi đã thay đổi",
      CONTEST_REGISTRATION_BODY: (contest: string, status: string) =>
        `${contest}: ${status}.`,
      CONTEST_STARTED_TITLE: "Cuộc thi đã bắt đầu",
      CONTEST_STARTED_BODY: (contest: string) => `${contest} đã chính thức bắt đầu.`,
      CONTEST_LIFECYCLE_TITLE: "Cuộc thi có cập nhật mới",
      CONTEST_LIFECYCLE_BODY: (contest: string, status: string) =>
        `${contest} hiện ở trạng thái ${status}.`,
      CONTEST_ANNOUNCEMENT_TITLE: "Thông báo mới từ ban tổ chức",
      CONTEST_ANNOUNCEMENT_BODY: (contest: string) =>
        `${contest} vừa có một thông báo mới.`,
      CONTEST_CLARIFICATION_TITLE: "Câu hỏi của bạn đã được trả lời",
      CONTEST_CLARIFICATION_BODY: (contest: string) =>
        `Ban tổ chức ${contest} đã phản hồi câu hỏi của bạn.`,
      CONTEST_FINAL_RANK_TITLE: "Kết quả cuộc thi đã sẵn sàng",
      CONTEST_FINAL_RANK_BODY: (contest: string, rank: string) =>
        `${contest}: bạn xếp hạng ${rank}.`,
      CONTEST_RATING_TITLE: "Điểm xếp hạng đã được cập nhật",
      CONTEST_RATING_BODY: (contest: string, delta: string) =>
        `${contest}: điểm xếp hạng thay đổi ${delta}.`,
      SECURITY_ACCOUNT_TITLE: "Tài khoản có thay đổi bảo mật",
      SECURITY_ACCOUNT_BODY:
        "Một thiết lập bảo mật quan trọng vừa được thay đổi. Hãy kiểm tra nếu đó không phải bạn.",
      SECURITY_SESSION_TITLE: "Phiên đăng nhập có thay đổi",
      SECURITY_SESSION_BODY: (device: string) =>
        `Trạng thái phiên đăng nhập trên ${device} vừa được cập nhật.`,
      AUTHORIZATION_CHANGED_TITLE: "Quyền truy cập đã được cập nhật",
      AUTHORIZATION_CHANGED_BODY:
        "Vai trò hoặc quyền sử dụng của bạn vừa thay đổi.",
      OPERATION_ALERT_TITLE: "Hệ thống chấm cần chú ý",
      OPERATION_ALERT_BODY: (summary: string) => summary,
      MATERIAL_PUBLISHED_TITLE: "Có tài liệu mới",
      MATERIAL_PUBLISHED_BODY: (title: string) => `${title} vừa được xuất bản.`,
    },
  },

  ONBOARDING_PROFILE: {
    TITLE: "Hồ sơ bắt buộc",
    DESCRIPTION:
      "Các trường bên dưới được đồng bộ trực tiếp từ cấu hình hiện hành của hệ thống.",
    LOADING: "Đang tải cấu trúc hồ sơ bắt buộc...",
    LOAD_ERROR_TITLE: "Không thể tải cấu trúc hồ sơ",
    LOAD_ERROR_DESCRIPTION:
      "Chưa thể xác minh các trường bắt buộc. Hãy tải lại trước khi tạo tài khoản để tránh gửi dữ liệu theo cấu trúc cũ.",
    REFRESHING:
      "Cấu trúc vừa thay đổi; đang đối chiếu lại và giữ các giá trị còn tương thích...",
    REFRESH_ERROR:
      "Chưa thể tải cấu trúc mới. Bản nháp vẫn được giữ an toàn; hãy thử lại trước khi tiếp tục.",
    REVISION: (revision: number) => `Phiên bản ${revision}`,
    SCHEMA_NOTE:
      "Hệ thống luôn đối chiếu cấu trúc mới nhất trước khi tạo tài khoản. Nếu biểu mẫu thay đổi trong lúc bạn nhập, các giá trị còn phù hợp sẽ được giữ lại để bạn kiểm tra.",
    SCHEMA_RELOADED:
      "Cấu trúc hồ sơ vừa thay đổi. Biểu mẫu đã được tải lại và chỉ giữ các giá trị còn tương thích; vui lòng kiểm tra trước khi gửi lại.",
    INVALID_VALUE:
      "Một giá trị chưa phù hợp với quy tắc mới nhất. Hãy kiểm tra trường được đánh dấu rồi thử lại.",
    COMPLETE_REQUIRED:
      "Vui lòng hoàn tất tất cả trường hồ sơ bắt buộc trước khi tiếp tục.",
    SUBMISSION_REVIEW_REQUIRED:
      "Biểu mẫu hồ sơ cần được kiểm tra lại trước khi gửi.",
  },

  // Auth
  AUTH: {
    // Tabs
    TAB_LOGIN: "Đăng Nhập",
    TAB_REGISTER: "Đăng Ký",
    LOGIN: "Đăng Nhập",
    REGISTER: "Đăng Ký",
    // Login
    LOGIN_TITLE: "Đạo Hữu Quay Lại!",
    LOGIN_SUBTITLE: "Nhập danh tính để tiếp tục con đường tu tiên.",
    LOGIN_BUTTON: "ĐĂNG NHẬP",
		LOGIN_INVALID: "Tên đăng nhập hoặc mật khẩu không đúng.",
		LOGIN_ERROR: "Chưa thể đăng nhập lúc này. Vui lòng thử lại.",
		REGISTER_ERROR: "Chưa thể tạo tài khoản lúc này. Vui lòng kiểm tra thông tin và thử lại.",
		OAUTH_PROVIDER_NAME: {
			google: "Google",
			github: "GitHub",
		},
		OAUTH_LOGIN_WITH: (provider: string) => `Đăng nhập bằng ${provider}`,
		OAUTH_CONNECTING: (provider: string) => `Đang kết nối ${provider}...`,
		OAUTH_START_ERROR: (provider: string) =>
			`Không thể bắt đầu đăng nhập bằng ${provider}. Vui lòng thử lại.`,
    OAUTH_CALLBACK_TITLE: "Đang xác minh danh tính",
    OAUTH_CALLBACK_DESCRIPTION:
      "Judgify đang hoàn tất phiên đăng nhập an toàn của bạn.",
    OAUTH_CALLBACK_FAILED_TITLE: "Không thể hoàn tất xác minh",
    OAUTH_CALLBACK_ERROR:
      "Phiên xác minh không thể hoàn tất. Vui lòng quay lại và bắt đầu đăng nhập lại.",
    OAUTH_PROVIDER_DENIED:
      "Yêu cầu đăng nhập đã bị hủy. Bạn có thể quay lại và thử lại khi sẵn sàng.",
    OAUTH_CALLBACK_INVALID:
      "Liên kết xác minh không hợp lệ hoặc thiếu thông tin cần thiết.",
		OAUTH_ONBOARDING_TITLE: "Chọn thiên mệnh khởi đầu",
		OAUTH_ONBOARDING_DESCRIPTION:
			"Kiểm tra hồ sơ bắt buộc và chọn đủ ba Thiên Phú trước khi hoàn tất tài khoản mới.",
		OAUTH_ONBOARDING_LOADING: "Đang tải thiên mệnh dành cho bạn...",
		OAUTH_ONBOARDING_COMPLETE: "Hoàn tất đăng nhập",
		OAUTH_ONBOARDING_SUBMITTING: "Đang hoàn tất...",
		OAUTH_ONBOARDING_COMPLETE_ERROR:
			"Chưa thể hoàn tất lựa chọn. Vui lòng thử lại; Judgify sẽ không tạo thêm tài khoản.",
		OAUTH_ONBOARDING_RETRY: "Thử lại",
    OAUTH_LINK_DESCRIPTION:
      "Judgify đang liên kết nhà cung cấp đăng nhập với tài khoản hiện tại của bạn.",
    OAUTH_LINK_SUCCESS: "Đã liên kết tài khoản đăng nhập an toàn.",
    OAUTH_BACK_TO_PROFILE: "Về hồ sơ bảo mật",
    // Fields
    USERNAME: "Pháp Danh",
    USERNAME_PLACEHOLDER: "Nhập pháp danh của bạn",
    USERNAME_REQUIREMENTS:
      "Dùng từ 3 đến 50 ký tự, chỉ gồm chữ cái không dấu và chữ số.",
    PASSWORD: "Mật Khẩu",
    PASSWORD_PLACEHOLDER: "Nhập mật khẩu",
    RECOVERY_EMAIL: "Email Khôi Phục",
    RECOVERY_EMAIL_PLACEHOLDER: "tenban@example.com",
    RECOVERY_EMAIL_DESCRIPTION:
      "Dùng để xác minh tài khoản và khôi phục quyền truy cập khi cần.",
    RECOVERY_EMAIL_INVALID: "Vui lòng nhập một địa chỉ email hợp lệ.",
    FORGOT_PASSWORD: "Quên Mật Khẩu?",
    NO_ACCOUNT: "Chưa có tài khoản?",
    HAS_ACCOUNT: "Đã có tài khoản?",
    FIRST_NAME: "Tên",
    LAST_NAME: "Họ",
    FIRST_NAME_PLACEHOLDER: "Văn A",
    LAST_NAME_PLACEHOLDER: "Nguyễn",
    BIRTHDAY: "Ngày Sinh",
    GENDER: "Giới Tính",
    GENDER_MALE: "Nam",
    GENDER_FEMALE: "Nữ",
    GENDER_OTHER: "Khác",
    // Register
    REGISTER_TITLE: "Gia Nhập Tu Viện",
    REGISTER_SUBTITLE: "Điền thông tin để bắt đầu hành trình tu luyện.",
    REGISTER_NEXT: "Tiếp Tục Tu Luyện",
    REGISTER_SUBMIT: "Xuất Sơn",
    REGISTER_BACK: "Quay Lại",
    REGISTER_MISSING_INFO: "Thiếu thông tin tiên nhân!",
    REGISTER_SUCCESS:
      "Đăng ký thành công! Hãy kiểm tra email để xác minh phương thức khôi phục.",
    // Trait Selection
    TRAIT_TITLE: "Khai Mở Thiên Mệnh",
    TRAIT_SUBTITLE: "Tung xúc xắc để nhận Căn Cốt và Thiên Phú ngẫu nhiên.",
    TRAIT_ROLL_BUTTON: "Tung Vận Mệnh",
    TRAIT_REROLL: "Tung Lại",
    TRAIT_ROLLING: "Đang quay...",
    TRAIT_ROOT_BONE: "Căn Cốt",
    TRAIT_TALENT: "Thiên Phú",
    TRAIT_SELECT_ROOT_BONE: "Chọn 1 Căn Cốt",
    TRAIT_SELECT_TALENTS: "Chọn 3 Thiên Phú",
    TRAIT_SELECTED: "Đã chọn",
    TRAIT_ROOT_BONE_REQUIRED: "Ngài phải chọn 1 Căn Cốt!",
    TRAIT_TALENTS_REQUIRED: "Vui lòng chọn đủ 3 Thiên Phú để xuất sơn.",
    TRAIT_MAX_TALENTS: "Chỉ được mang theo tối đa 3 Thiên Phú!",
    TRAIT_FETCH_ERROR: "Không thể tải danh sách thiên mệnh.",
    TRAIT_ROLL_EXPIRES: "Lựa chọn có hiệu lực đến",
    TRAIT_ROLL_REMAINING: (count: number) => `Còn ${count} lượt tung lại`,
    TRAIT_ROLL_EXPIRED: "Thiên mệnh đã hết hiệu lực. Vui lòng tung lại.",
    TRAIT_ROLL_EXHAUSTED: "Đã dùng hết lượt tung trong phiên đăng ký này.",
    // Progress steps
    STEP_INFO: "Phàm Trần",
    STEP_TRAITS: "Thiên Mệnh",
    // Other
    CONFIRM_PASSWORD: "Xác Nhận Mật Khẩu",
    CONFIRM_PASSWORD_PLACEHOLDER: "Nhập lại mật khẩu",
    PASSWORD_MISMATCH: "Mật khẩu xác nhận không khớp",
    WELCOME_BACK: "Chào Mừng Trở Lại",
    WELCOME_SUBTITLE: "Hãy đăng nhập để tiếp tục hành trình tu luyện",
    FORGOT_PASSWORD_TITLE: "Quên Mật Khẩu",
    FORGOT_PASSWORD_SUBTITLE: "Nhập pháp danh để khôi phục mật khẩu",
    FORGOT_PASSWORD_SUCCESS:
      "Nếu tài khoản này có email khôi phục đã xác minh, hướng dẫn đặt lại mật khẩu sẽ được gửi đến địa chỉ đó.",
    FORGOT_PASSWORD_FAILED:
      "Chưa thể tiếp nhận yêu cầu lúc này. Vui lòng thử lại sau.",
    BACK_TO_LOGIN: "Quay lại đăng nhập",
    SEND_RESET_LINK: "Gửi Liên Kết Khôi Phục",
    RESET_PASSWORD_TITLE: "Đặt Lại Mật Khẩu",
    RESET_PASSWORD_SUBTITLE: "Nhập mật khẩu mới cho tài khoản của bạn",
    NEW_PASSWORD: "Mật Khẩu Mới",
    NEW_PASSWORD_PLACEHOLDER: "Nhập mật khẩu mới",
    RESET_PASSWORD: "Đặt Lại Mật Khẩu",
    RESET_PASSWORD_SUCCESS: "Đặt lại mật khẩu thành công!",
    RESET_PASSWORD_FAILED:
      "Không thể đặt lại mật khẩu lúc này. Vui lòng thử lại.",
    RESET_LINK_PREPARING: "Đang mở liên kết bảo mật...",
    RESET_LINK_INVALID:
      "Liên kết đặt lại mật khẩu không hợp lệ, đã hết hạn hoặc đã được sử dụng.",
    PASSWORD_REQUIREMENTS: "Mật khẩu phải có từ 8 đến 256 ký tự.",
    SHOW_PASSWORD: "Hiện mật khẩu",
    HIDE_PASSWORD: "Ẩn mật khẩu",
    RESET_REDIRECTING: "Đang chuyển hướng đến trang đăng nhập...",
    INVITATION_ACTIVATION_EYEBROW: "Lời mời bảo mật",
    INVITATION_ACTIVATION_TITLE: "Hoàn Tất Tài Khoản",
    INVITATION_ACTIVATION_SUBTITLE:
      "Tự đặt mật khẩu để kích hoạt tài khoản Judgify đã được mời cho bạn.",
    INVITATION_ACTIVATION_SECURITY_NOTE:
      "Liên kết chỉ dùng một lần. Mật khẩu do chính bạn thiết lập và không hiển thị cho quản trị viên.",
    INVITATION_ACTIVATION_PREPARING: "Đang mở lời mời bảo mật...",
    INVITATION_ACTIVATION_SUBMIT: "Kích Hoạt Tài Khoản",
    INVITATION_ACTIVATION_SUBMITTING: "Đang kích hoạt...",
    INVITATION_ACTIVATION_SUCCESS_TITLE: "Tài khoản đã sẵn sàng",
    INVITATION_ACTIVATION_SUCCESS_DESCRIPTION:
      "Mật khẩu đã được thiết lập an toàn. Bạn có thể đăng nhập và bắt đầu sử dụng Judgify.",
    INVITATION_ACTIVATION_LOGIN: "Tiếp Tục Đăng Nhập",
    INVITATION_ACTIVATION_INVALID:
      "Lời mời không hợp lệ, đã hết hạn hoặc đã được sử dụng. Hãy liên hệ quản trị viên để được gửi lại lời mời mới.",
    INVITATION_ACTIVATION_FAILED:
      "Chưa thể kích hoạt tài khoản lúc này. Lời mời vẫn được giữ an toàn trong phiên hiện tại để bạn thử lại.",
    VERIFY_RECOVERY_CONTACT_TITLE: "Xác Minh Email Khôi Phục",
    VERIFY_RECOVERY_CONTACT_SUBTITLE:
      "Judgify đang kiểm tra liên kết bảo mật cho tài khoản của bạn.",
    VERIFY_RECOVERY_CONTACT_PREPARING: "Đang xác minh liên kết bảo mật...",
    VERIFY_RECOVERY_CONTACT_SUCCESS_TITLE: "Email đã được xác minh",
    VERIFY_RECOVERY_CONTACT_SUCCESS_DESCRIPTION:
      "Tài khoản của bạn giờ đã có phương thức khôi phục an toàn.",
    VERIFY_RECOVERY_CONTACT_INVALID:
      "Liên kết xác minh không hợp lệ, đã hết hạn hoặc đã được sử dụng.",
    VERIFY_RECOVERY_CONTACT_FAILED:
      "Chưa thể xác minh email lúc này. Liên kết vẫn được giữ trong phiên hiện tại để bạn thử lại.",
    VERIFY_RECOVERY_CONTACT_RETRY: "Thử Xác Minh Lại",
    VERIFY_RECOVERY_CONTACT_LOGIN: "Tiếp Tục Đăng Nhập",
    LOGIN_SUCCESS: "Đăng nhập thành công!",
    DIVIDER_OR: "hoặc",
    SESSION_EXPIRED: "Phiên đăng nhập đã hết hạn",
    ADMIN_INVALID_CREDENTIALS: "Tên đăng nhập hoặc mật khẩu không đúng",
    ADMIN_TITLE: "Admin Panel",
    ADMIN_SUBTITLE: "Đăng nhập để truy cập bảng điều khiển",
    ADMIN_USERNAME: "Tên đăng nhập",
    ADMIN_USERNAME_PLACEHOLDER: "admin",
    ADMIN_PASSWORD: "Mật khẩu",
    ADMIN_VERIFYING: "Đang xác thực...",
    ADMIN_PRODUCT_NAME: "Judgify Administration Panel",
  },

  ABOUT: {
    BRAND: "Judgify",
    MOTTO: "Vạn Cổ Thống Trị - Nhất Lộ Duy Tâm",
    GREETING: "Kính thưa chư vị đạo hữu...",
    INTRO:
      "Tại hạ giới bao la này, giữa dòng đời cuồn cuộn như thác đổ, Judgify mọc lên như một ngọn cô phong giữa biển mây. Đây không chỉ là nơi hội tụ của những linh hồn đam mê mật mã, mà là một Thánh địa liễu ngộ, nơi chân lý không nằm ở những trang kinh thư khô khan, mà nằm trong từng nhịp gõ của trí tuệ, trong từng hơi thở của logic.",
    WAY:
      "Người xưa có câu: \"Nghịch thiên nhi hành, vị chi tu tiên\". Kẻ sĩ luyện code chúng ta cũng vậy. Mỗi một thuật toán thâm sâu chính là một đạo chân ngôn. Mỗi một dòng mã lệnh thực thi chính là một lần vận chuyển linh khí trong kinh mạch. Thế giới này vốn dĩ hỗn độn, và chúng ta—những tu chân giả của thời đại mới—dùng trật tự của logic để định hình lại càn khôn.",
    CHALLENGE_PREFIX:
      "Hãy nhìn vào những bài tập tại Arena, chúng không đơn thuần là những câu đố. Đó là những Thiên Kiếp mà mỗi đạo hữu phải đối mặt trên con đường phi thăng. Một khi ấn tích",
    ACCEPTED: "Accepted",
    CHALLENGE_MIDDLE:
      "hiện lên, ấy là lúc mây mờ tan biến, linh đài sáng rực, tu vi tinh tiến một bậc. Nhưng chớ có nản lòng khi chạm phải",
    WRONG_ANSWER: "Wrong Answer",
    CHALLENGE_SUFFIX:
      "bởi đó chính là tâm ma trỗi dậy để thử thách bản ngã. Kẻ mạnh thực sự không phải kẻ không bao giờ thất bại, mà là kẻ dùng thất bại làm đá mài cho thanh kiếm trí tuệ thêm phần sắc bén.",
    PATHS:
      "Đạo lý vốn hữu hình mà vô ảnh. Dù chư vị chọn con đường nào—là sự thần tốc của C, sự biến hóa của Python hay sự vững chãi của Go—thì đích đến cuối cùng vẫn là sự thuần khiết của tư duy. Tại Tàng Kinh Các, chư vị tìm thấy tri thức. Tại Arena, chư vị tìm thấy thực tại. Và tại Bảng Phong Thần, chư vị sẽ tìm thấy vinh quang vĩnh cửu.",
    REMINDER:
      "Nhưng hãy nhớ lấy: Vinh quang chỉ là mây khói, bản ngã mới là trường tồn. Chớ mưu cầu hư danh mà quên mất đạo tâm ban sơ. Hãy để mỗi dòng code là một lời cầu nguyện, mỗi thuật toán là một sự tri ân đối với vẻ đẹp của vũ trụ toán học.",
    JOURNEY:
      "Ngày mà chư vị đứng trên đỉnh cao của Vạn Cổ Tiên Bảng, ngắm nhìn chúng sinh đang mải miết dưới chân, lúc đó chư vị sẽ hiểu rằng: Hóa ra, hành trình vạn dặm khởi đầu từ một hàm `main()`, và sự trường sinh nằm ở niềm đam mê bất diệt với logic.",
    BLESSING_TITLE: "Đạo Pháp Tự Nhiên - Thiên Địa Trường Tồn",
    BLESSING:
      "Kính chúc chư vị đạo hữu sớm ngày đắc đạo, danh chấn thiên hạ.",
  },

  // Profile
  PROFILE: {
    TITLE: "Hồ Sơ Tu Luyện",
    SUBTITLE: "Thông tin cá nhân và tiến trình tu luyện",
    AUTH_LOADING: "Đang xác minh phiên đăng nhập...",
    AUTH_ERROR_TITLE: "Không thể xác minh phiên đăng nhập",
    AUTH_ERROR_DESCRIPTION:
      "Kết nối xác thực đang gián đoạn. Hãy thử lại trước khi mở hồ sơ.",
    AUTH_RETRY: "Xác Minh Lại",
    LOGIN_REQUIRED_TITLE: "Đăng nhập để xem hồ sơ",
    LOGIN_REQUIRED_DESCRIPTION:
      "Hồ sơ, tiến trình và cài đặt bảo mật chỉ hiển thị cho chủ tài khoản.",
    LOGIN_ACTION: "Đăng Nhập",
    LOADING: "Đang tải hồ sơ tu luyện...",
    UNPROVISIONED_TITLE: "Tài khoản vận hành đã sẵn sàng",
    UNPROVISIONED_DESCRIPTION:
      "Quản lý danh tính, quyền riêng tư và bảo mật mà không cần dữ liệu tu luyện giả.",
    UNPROVISIONED_BADGE: "Hồ sơ tu luyện chưa khởi tạo",
    UNPROVISIONED_NOTE:
      "Tài khoản này chưa đi qua luồng nhập môn nên chưa có cảnh giới, chỉ số hay bộ thiên phú. Các khu vực quản trị và bảo mật vẫn hoạt động bình thường; dữ liệu trò chơi chỉ xuất hiện sau một quy trình cấp phát hợp lệ.",
    DIFFICULTIES_LOADING: "Đang đồng bộ danh mục độ khó...",
    DIFFICULTIES_LOAD_ERROR:
      "Danh mục độ khó chưa tải được; thống kê đã giải vẫn được giữ nguyên.",
    DIFFICULTIES_RETRY: "Tải Lại",
    EDIT_PROFILE: "Chỉnh Sửa Hồ Sơ",
    EDIT_DESCRIPTION:
      "Cập nhật thông tin hiển thị của bạn. Những thay đổi sẽ được áp dụng ngay sau khi lưu.",
    SAVE_CHANGES: "Lưu Thay Đổi",
    SAVING: "Đang lưu...",
    CANCEL_EDIT: "Hủy",
    UPDATE_SUCCESS: "Cập nhật hồ sơ thành công!",
    UPDATE_ERROR: "Không thể cập nhật hồ sơ. Vui lòng thử lại.",
    FORM_REQUIRED: "Thông tin này là bắt buộc.",
    FORM_NAME_TOO_LONG: "Họ và tên không được vượt quá 50 ký tự.",
    FORM_BIRTHDAY_INVALID: "Ngày sinh không hợp lệ.",
    FORM_BIRTHDAY_FUTURE: "Ngày sinh không thể ở trong tương lai.",
    ATTRIBUTES: {
      DESCRIPTION:
        "Chỉnh sửa các trường hồ sơ đang hoạt động. Kiểu dữ liệu và quy tắc kiểm tra được đồng bộ trực tiếp từ cấu hình hệ thống.",
      SCHEMA_NOTE:
        "Mỗi thay đổi được đối chiếu với đúng phiên bản cấu trúc bạn đang xem. Nếu quản trị viên vừa cập nhật cấu trúc hoặc giá trị đã đổi ở nơi khác, hệ thống sẽ yêu cầu tải lại thay vì ghi đè.",
      LOADING: "Đang đồng bộ cấu trúc hồ sơ...",
      LOAD_ERROR_TITLE: "Không thể tải cấu trúc hồ sơ",
      LOAD_ERROR_DESCRIPTION:
        "Cấu trúc hồ sơ đang tạm thời không khả dụng. Vui lòng tải lại trước khi chỉnh sửa.",
      IDENTITY_MISMATCH:
        "Phiên hồ sơ không còn khớp với tài khoản hiện tại. Vui lòng tải lại trang.",
      EMPTY: "Hiện chưa có trường hồ sơ nào có thể hiển thị.",
      REQUIRED: "Bắt buộc",
      READ_ONLY: "Chỉ đọc",
      UNSET: "Xóa giá trị",
      NOT_SET: "Chưa thiết lập",
      BOOLEAN_TRUE: "Đang bật",
      BOOLEAN_FALSE: "Đang tắt",
      SELECT_PLACEHOLDER: "Chọn một giá trị",
      VALUE_PLACEHOLDER: "Nhập giá trị",
      OPTION_LABEL: (value: string) => value,
      NO_CHANGES: "Không có thay đổi nào cần lưu.",
      CONFLICT:
        "Hồ sơ hoặc cấu trúc trường vừa thay đổi. Hãy tải lại biểu mẫu rồi thử lại để tránh ghi đè dữ liệu mới.",
      RELOAD_SCHEMA: "Tải Lại Biểu Mẫu",
      FIELD_ERROR: (
        error:
          | "required"
          | "invalid"
          | "minimum"
          | "maximum"
          | "min_length"
          | "max_length"
          | "enum"
          | "pattern"
          | "payload_size",
      ) => {
        if (error === "required") return "Trường này là bắt buộc."
        if (error === "minimum") return "Giá trị nhỏ hơn mức tối thiểu cho phép."
        if (error === "maximum") return "Giá trị lớn hơn mức tối đa cho phép."
        if (error === "min_length") return "Nội dung chưa đạt độ dài tối thiểu."
        if (error === "max_length") return "Nội dung vượt quá độ dài tối đa."
        if (error === "enum") return "Giá trị không thuộc danh sách được cho phép."
        if (error === "pattern") return "Giá trị chưa đúng định dạng yêu cầu."
        if (error === "payload_size") return "Giá trị vượt quá dung lượng cho phép."
        return "Giá trị không hợp lệ."
      },
    },
    LOAD_ERROR_TITLE: "Không thể tải hồ sơ",
    LOAD_ERROR_DESCRIPTION:
      "Dữ liệu hồ sơ đang tạm thời không khả dụng. Phiên đăng nhập của bạn vẫn được giữ nguyên.",
    JOINED_DATE: "Ngày Gia Nhập",
    BIRTHDAY: "Đản Thần",
    RANK: "Cấp Bậc",
    CULTIVATION_PROGRESS: "Tu Luyện Tiến Trình",
    CULTIVATION_SUBTITLE: "Cảnh giới, cấp bậc và linh căn ngũ hành",
    CULTIVATION_REALM: "Cảnh Giới",
    CULTIVATION_RANK: "Cấp Bậc",
    CULTIVATION_EXP_TO_BREAK: "EXP để đột phá",
    CULTIVATION_RATING_TO_NEXT: "Rating để thăng cảnh",
    CULTIVATION_NO_ELEMENTS: "Chưa có dữ liệu ngũ hành",
    CULTIVATION_ELEMENTS_TITLE: "Phân bố kinh nghiệm ngũ hành",
    CULTIVATION_ELEMENTS_DESCRIPTION:
      "Biểu đồ so sánh kinh nghiệm giữa các nguyên tố; giá trị chính xác được liệt kê bên dưới.",
    CULTIVATION_MAX_LEVEL: "Đỉnh Phong",
    CULTIVATION_MAX_RANK: "Vô Song",
    CULTIVATION_NEXT: "→",
    TRAITS_TITLE: "Căn Cốt & Thiên Phú",
    TRAITS_SUBTITLE: "Thể chất bẩm sinh và tài năng thiên bẩm",
    TALENTS_LABEL: "Thiên Phú",
    NO_ROOT_BONE: "Chưa có căn cốt",
    NO_TALENTS: "Chưa có thiên phú",
    REWARD_PROFILE: {
      TITLE: "Hồ Sơ Phần Thưởng",
      DESCRIPTION:
        "Căn cốt, thiên phú và hiệu ứng đang được dùng để tính EXP cho từng lần giải bài.",
      LOADING: "Đang tải hồ sơ phần thưởng...",
      LOAD_ERROR: "Chưa thể tải hồ sơ phần thưởng.",
      LOAD_ERROR_DESCRIPTION:
        "Tiến trình hiện tại vẫn an toàn. Hãy đồng bộ lại riêng khu vực này.",
      RETRY: "Đồng Bộ Lại",
      REVISION: (value: number) => `Phiên bản ${value}`,
      EFFECTIVE_AT: "Có hiệu lực từ",
      CHECKSUM: "Mã kiểm chứng",
      SOURCE: "Nguồn khởi tạo",
      SOURCE_ONBOARDING: "Đăng ký tài khoản",
      SOURCE_ADMINISTRATIVE: "Điều chỉnh quản trị",
    },
    EDIT_TOOLTIP: "Chỉnh sửa hồ sơ",
    LOGOUT_TOOLTIP: "Đăng xuất",
    PROBLEMS_SOLVED: "Bài Đã Giải",
    CONTESTS_JOINED: "Đại Hội Tham Gia",
    PERSONAL_INFO: "Thông Tin Cá Nhân",
    STATISTICS: "Thống Kê Tu Luyện",
    EASY_SOLVED: "Dễ",
    MEDIUM_SOLVED: "Trung",
    HARD_SOLVED: "Khó",
    TOTAL_SUBMISSIONS: "Tổng Nộp Bài",
    ACCEPTANCE_RATE: "Tỉ Lệ AC",
    CHANGE_PASSWORD: {
      TITLE: "Đổi Mật Khẩu",
      DESCRIPTION:
        "Dùng mật khẩu hiện tại để đặt mật khẩu mới. Tất cả phiên đăng nhập sẽ được thu hồi để bảo vệ tài khoản.",
      CURRENT_PASSWORD: "Mật khẩu hiện tại",
      CURRENT_PASSWORD_PLACEHOLDER: "Nhập mật khẩu hiện tại",
      NEW_PASSWORD: "Mật khẩu mới",
      NEW_PASSWORD_PLACEHOLDER: "Nhập mật khẩu mới",
      CONFIRM_PASSWORD: "Xác nhận mật khẩu mới",
      CONFIRM_PASSWORD_PLACEHOLDER: "Nhập lại mật khẩu mới",
      SUBMIT: "Đổi Mật Khẩu",
      PROCESSING: "Đang đổi mật khẩu...",
      SUCCESS_TITLE: "Đã đổi mật khẩu",
      SUCCESS_DESCRIPTION:
        "Các phiên đăng nhập hiện có đã được thu hồi. Hãy đăng nhập lại bằng mật khẩu mới.",
      LOGIN_AGAIN: "Đăng Nhập Lại",
      CURRENT_PASSWORD_ERROR: "Mật khẩu hiện tại không chính xác.",
      ACTION_ERROR: "Chưa thể đổi mật khẩu lúc này. Vui lòng thử lại.",
    },
    RECOVERY_CONTACT: {
      TITLE: "Bảo Mật Tài Khoản",
      DESCRIPTION:
        "Quản lý email dùng để xác minh danh tính và khôi phục quyền truy cập.",
      CURRENT_LABEL: "Email khôi phục hiện tại",
      VERIFIED: "Đã xác minh",
      NO_CURRENT: "Chưa có email khôi phục đã xác minh",
      NO_CURRENT_DESCRIPTION:
        "Hãy hoàn tất liên kết xác minh đang chờ để bảo vệ tài khoản của bạn.",
      PENDING_TITLE: "Đang chờ xác minh",
      PENDING_REGISTRATION:
        "Xác minh email đăng ký ban đầu để kích hoạt phương thức khôi phục.",
      PENDING_REPLACEMENT:
        "Email hiện tại vẫn được giữ cho đến khi địa chỉ mới được xác minh.",
      DELIVERY_LABEL: "Trạng thái gửi",
      DELIVERY_QUEUED: "Đang xếp hàng",
      DELIVERY_SENT: "Đã gửi",
      DELIVERY_FAILED: "Gửi chưa thành công",
      EXPIRES_LABEL: "Liên kết hết hạn",
      REAUTHENTICATED: "Phiên bảo mật đã xác nhận",
      REAUTHENTICATED_UNTIL: (value: string) => `Có hiệu lực đến ${value}`,
      REPLACE: "Thay Đổi Email",
      RESEND: "Gửi Lại Liên Kết",
      RESEND_AFTER: (value: string) => `Gửi lại sau ${value}`,
      CANCEL_PENDING: "Hủy Email Đang Chờ",
      REMOVE: "Xóa Email Hiện Tại",
      REMOVE_DISABLED:
        "Không thể xóa email trong trạng thái tài khoản hiện tại. Hãy hoàn tất hoặc hủy thay đổi đang chờ, hoặc giữ phương thức khôi phục bắt buộc cho tài khoản mật khẩu.",
      LOAD_ERROR_TITLE: "Không thể tải cài đặt bảo mật",
      LOAD_ERROR_DESCRIPTION:
        "Hồ sơ của bạn vẫn hoạt động bình thường. Hãy thử tải lại riêng phần bảo mật.",
      RETRY: "Tải Lại Bảo Mật",
      REAUTH_TITLE: "Xác Nhận Lại Danh Tính",
      REAUTH_DESCRIPTION:
        "Nhập mật khẩu hiện tại trước khi thay đổi thông tin khôi phục nhạy cảm.",
      CURRENT_PASSWORD: "Mật khẩu hiện tại",
      CURRENT_PASSWORD_PLACEHOLDER: "Nhập mật khẩu hiện tại",
      REAUTH_SUBMIT: "Xác Nhận",
      REAUTH_SUCCESS: "Đã xác nhận phiên bảo mật.",
      REAUTH_ERROR: "Mật khẩu không đúng hoặc chưa thể xác nhận lúc này.",
      REAUTH_REQUIRED:
        "Phiên xác nhận đã hết hiệu lực. Vui lòng nhập lại mật khẩu.",
      REPLACE_TITLE: "Thay Đổi Email Khôi Phục",
      REPLACE_DESCRIPTION:
        "Judgify sẽ gửi liên kết xác minh đến địa chỉ mới. Email hiện tại chỉ được thay thế sau khi xác minh thành công.",
      NEW_EMAIL: "Email khôi phục mới",
      NEW_EMAIL_PLACEHOLDER: "tenban@example.com",
      NEW_EMAIL_HELP:
        "Địa chỉ đầy đủ chỉ được gửi đến máy chủ cho thao tác này và không được lưu trong trình duyệt.",
      NEW_EMAIL_INVALID: "Vui lòng nhập một địa chỉ email hợp lệ.",
      REPLACE_SUBMIT: "Gửi Liên Kết Xác Minh",
      REPLACE_SUCCESS: "Đã gửi liên kết xác minh đến email mới.",
      RESEND_SUCCESS: "Đã gửi lại liên kết xác minh.",
      CANCEL_TITLE: "Hủy Email Thay Thế?",
      CANCEL_DESCRIPTION:
        "Email hiện tại vẫn được giữ nguyên. Liên kết xác minh cho địa chỉ đang chờ sẽ mất hiệu lực.",
      CANCEL_CONFIRM: "Hủy Email Đang Chờ",
      CANCEL_SUCCESS: "Đã hủy email thay thế đang chờ.",
      REMOVE_TITLE: "Xóa Email Khôi Phục?",
      REMOVE_DESCRIPTION:
        "Email này sẽ không còn được dùng để xác minh hoặc khôi phục tài khoản. Chỉ tiếp tục khi bạn đã có phương thức đăng nhập an toàn khác.",
      REMOVE_CONFIRM: "Xóa Email Khôi Phục",
      REMOVE_SUCCESS: "Đã xóa email khôi phục.",
      ACTION_ERROR: "Chưa thể hoàn tất thao tác. Vui lòng thử lại.",
      VERSION_CONFLICT:
        "Trạng thái bảo mật vừa thay đổi ở nơi khác. Dữ liệu mới nhất đang được tải lại.",
      RATE_LIMITED:
        "Bạn đã thực hiện quá nhiều yêu cầu. Vui lòng chờ rồi thử lại.",
      PROCESSING: "Đang xử lý...",
    },
    SESSIONS: {
      TITLE: "Các Phiên Đăng Nhập",
      DESCRIPTION:
        "Kiểm tra nơi tài khoản đang hoạt động và thu hồi những phiên bạn không nhận ra.",
      CURRENT: "Phiên hiện tại",
      ACTIVE: "Đang hoạt động",
      REVOKED: "Đã thu hồi",
      EXPIRED: "Đã hết hạn",
      UNKNOWN_DEVICE: "Thiết bị không xác định",
      CREATED: "Bắt đầu",
      LAST_SEEN: "Hoạt động gần nhất",
      REAUTHENTICATED: "Xác thực gần nhất",
      IDLE_EXPIRES: "Hết hạn do không hoạt động",
      EXPIRES: "Hết hạn",
      REVOKE: "Thu hồi phiên",
      REVOKE_TITLE: "Thu hồi phiên đăng nhập?",
      REVOKE_DESCRIPTION:
        "Thiết bị này sẽ không thể tiếp tục sử dụng phiên đăng nhập đã chọn.",
      CURRENT_REVOKE_DESCRIPTION:
        "Phiên hiện tại trên thiết bị này sẽ bị thu hồi và bạn sẽ được chuyển tới trang đăng nhập. Các thiết bị khác vẫn duy trì phiên đăng nhập.",
      REVOKE_CONFIRM: "Thu hồi phiên",
      REVOKE_SUCCESS: "Đã thu hồi phiên đăng nhập.",
      LOGOUT_ALL: "Đăng xuất mọi nơi",
      LOGOUT_ALL_TITLE: "Đăng xuất khỏi mọi thiết bị?",
      LOGOUT_ALL_DESCRIPTION:
        "Tất cả phiên đăng nhập sẽ bị thu hồi. Bạn sẽ cần đăng nhập lại trên thiết bị này.",
      LOGOUT_ALL_CONFIRM: "Đăng xuất mọi nơi",
      LOGOUT_ALL_SUCCESS: "Đã đăng xuất khỏi mọi thiết bị.",
      LOAD_ERROR_TITLE: "Không thể tải các phiên đăng nhập",
      LOAD_ERROR_DESCRIPTION:
        "Thông tin phiên chưa sẵn sàng. Tài khoản của bạn vẫn được giữ nguyên.",
      ACTION_ERROR: "Chưa thể hoàn tất thao tác bảo mật. Vui lòng thử lại.",
      NOT_FOUND: "Phiên này không còn tồn tại hoặc đã được thu hồi.",
      EMPTY: "Chưa có phiên đăng nhập nào để hiển thị.",
      PREVIOUS: "Trang trước",
      NEXT: "Trang tiếp",
      PAGE_LABEL: "Phân trang phiên đăng nhập",
      LOADING: "Đang tải các phiên đăng nhập...",
      RETRY: "Tải lại phiên",
      SECURITY_NOTE:
        "Thông tin nhạy cảm như địa chỉ IP và mã thông báo không được hiển thị tại đây.",
      BROWSER_EDGE: "Edge",
      BROWSER_OPERA: "Opera",
      BROWSER_FIREFOX: "Firefox",
      BROWSER_CHROME: "Chrome",
      BROWSER_SAFARI: "Safari",
      BROWSER_GENERIC: "Trình duyệt",
      PLATFORM_ANDROID: "Android",
      PLATFORM_IOS: "iOS",
      PLATFORM_MACOS: "macOS",
      PLATFORM_WINDOWS: "Windows",
      PLATFORM_LINUX: "Linux",
      PLATFORM_WEB: "Web",
    },
		PRIVACY: {
			TITLE: "Quyền Riêng Tư Hồ Sơ",
			DESCRIPTION:
				"Chọn liệu hồ sơ tối giản của bạn có thể được mở bằng đường dẫn công khai hay không.",
			PUBLIC_LABEL: "Hồ sơ công khai",
			PUBLIC_DESCRIPTION:
				"Mọi người có đường dẫn đều có thể xem tên tài khoản, ngày gia nhập và các trường được quản trị viên đánh dấu công khai.",
			PRIVATE_LABEL: "Chỉ mình bạn",
			PRIVATE_DESCRIPTION:
				"Đường dẫn công khai trả về cùng trạng thái không tìm thấy như một tài khoản không tồn tại.",
			ENABLE: "Cho phép mở hồ sơ công khai",
			DISABLE: "Đặt hồ sơ về chế độ riêng tư",
			LOADING: "Đang đồng bộ quyền riêng tư...",
			LOAD_ERROR_TITLE: "Không thể tải quyền riêng tư",
			LOAD_ERROR_DESCRIPTION:
				"Hệ thống chưa xác nhận được thiết lập hiện tại. Hồ sơ không được tự động công khai.",
			RETRY: "Tải Lại Thiết Lập",
			UPDATING: "Đang lưu thiết lập...",
			PUBLIC_SUCCESS: "Hồ sơ công khai đã được bật.",
			PRIVATE_SUCCESS: "Hồ sơ đã được chuyển về riêng tư.",
			CONFLICT:
				"Thiết lập vừa thay đổi ở một nơi khác. Phiên bản mới nhất đang được tải lại.",
			UPDATE_ERROR:
				"Chưa xác nhận được kết quả. Hãy thử lại; Judgify sẽ dùng lại đúng yêu cầu để không tạo thay đổi trùng.",
			VIEW_PUBLIC: "Xem Hồ Sơ Công Khai",
			UPDATED_AT: (value: string) => `Cập nhật gần nhất ${value}`,
			SAFETY_NOTE:
				"Email, vai trò, trạng thái bảo mật, revision nội bộ và các trường riêng tư không bao giờ xuất hiện trong hồ sơ công khai.",
		},
		OAUTH_ACCOUNTS: {
			TITLE: "Tài Khoản Đăng Nhập Liên Kết",
			DESCRIPTION:
				"Kết nối Google hoặc GitHub để đăng nhập nhanh mà không chia sẻ mật khẩu với Judgify.",
			CONNECTED: "Đã liên kết",
			NOT_CONNECTED: "Chưa liên kết",
			CONNECT: "Liên Kết",
			CONNECTING: "Đang chuyển hướng...",
			DISCONNECT: "Gỡ Liên Kết",
			DISCONNECT_TITLE: "Gỡ tài khoản đăng nhập?",
			DISCONNECT_DESCRIPTION: (provider: string) =>
				`Bạn sẽ không thể đăng nhập bằng ${provider} cho đến khi liên kết lại.`,
			DISCONNECT_CONFIRM: "Gỡ Liên Kết",
			LINKED_AT: "Liên kết lúc",
			LAST_USED: "Xác thực gần nhất",
			ACCOUNT_LABEL: "Tài khoản nhà cung cấp",
			LOAD_ERROR_TITLE: "Không thể tải tài khoản liên kết",
			LOAD_ERROR_DESCRIPTION:
				"Cài đặt đăng nhập đang tạm thời không khả dụng. Hãy tải lại riêng khu vực này.",
			RETRY: "Tải Lại",
			CONNECT_ERROR: "Chưa thể bắt đầu liên kết. Vui lòng thử lại.",
			DISCONNECT_ERROR: "Chưa thể gỡ liên kết. Vui lòng thử lại.",
			LAST_METHOD_ERROR:
				"Không thể gỡ phương thức đăng nhập cuối cùng. Hãy đặt mật khẩu hoặc liên kết nhà cung cấp khác trước.",
			NOT_FOUND: "Liên kết này không còn tồn tại. Danh sách mới nhất đang được tải lại.",
			DISCONNECT_SUCCESS: (provider: string) => `Đã gỡ liên kết ${provider}.`,
			SECURITY_NOTE:
				"Judgify chỉ lưu định danh cần thiết; mã truy cập của nhà cung cấp không được lưu trong trình duyệt.",
		},
  },

	PUBLIC_PROFILE: {
		EYEBROW: "Hồ sơ công khai",
		JOINED: "Gia nhập Judgify",
		ATTRIBUTES_TITLE: "Thông Tin Được Chia Sẻ",
		ATTRIBUTES_DESCRIPTION:
			"Chỉ những trường đang hoạt động và được cấu hình công khai mới xuất hiện tại đây.",
		EMPTY_TITLE: "Chưa có thông tin công khai",
		EMPTY_DESCRIPTION:
			"Tài khoản này đang công khai hồ sơ nhưng chưa chia sẻ thêm trường thông tin nào.",
		BOOLEAN_TRUE: "Có",
		BOOLEAN_FALSE: "Không",
		LOADING: "Đang tải hồ sơ công khai...",
		NOT_FOUND_TITLE: "Không tìm thấy hồ sơ công khai",
		NOT_FOUND_DESCRIPTION:
			"Tài khoản không tồn tại, chưa hoạt động hoặc chủ tài khoản đang để hồ sơ ở chế độ riêng tư.",
		LOAD_ERROR_TITLE: "Chưa thể mở hồ sơ",
		LOAD_ERROR_DESCRIPTION:
			"Kết nối dữ liệu đang gián đoạn. Hãy thử tải lại sau ít giây.",
		RETRY: "Tải Lại Hồ Sơ",
		BACK_TO_ARENA: "Về Luyện Tập",
	},

  STATS: {
    TITLE: "Thống Kê Tu Luyện",
    SUBTITLE: "Tổng quan bài đã giải, tỉ lệ AC và phân bổ theo độ khó",
    TOTAL_SOLVED: "Bài Đã Giải",
    TOTAL_SUBMISSIONS: "Tổng Nộp Bài",
    AC_RATE: "Tỉ Lệ AC",
    BY_DIFFICULTY: "Theo Độ Khó",
    BY_TAG: "Theo Chủ Đề",
    NO_STATS: "Chưa có thống kê — hãy nộp bài đầu tiên!",
    BAI: "bài",
    TAGS_TITLE: "Chủ Đề Đã Chinh Phục",
    TAGS_SUBTITLE: "Các chủ đề đã giải được, phân loại theo ngũ hành linh căn",
    NO_TAGS: "Chưa chinh phục chủ đề nào",
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
    NO_FILTERED_PROBLEMS: "Không tìm thấy bài tập phù hợp",
    NO_FILTERED_PROBLEMS_DESC:
      "Hãy thử từ khóa khác hoặc xóa bớt bộ lọc để mở rộng kết quả.",
    CLEAR_EMPTY_FILTERS: "Xóa bộ lọc",
    PROBLEMS_LOADING: "Đang tải danh sách bài tập...",
    PROBLEMS_REFRESHING: "Đang cập nhật kết quả...",
    RESULTS_SUMMARY: (count: number) => `${count} bài tập phù hợp`,
    LOAD_ERROR_TITLE: "Không thể tải danh sách bài tập",
    LOAD_ERROR_DESCRIPTION:
      "Dữ liệu bài tập đang tạm thời không khả dụng. Bộ lọc của bạn vẫn được giữ nguyên.",
    FILTERS_LOADING: "Đang tải bộ lọc...",
    FILTERS_LOAD_ERROR_TITLE: "Một số bộ lọc chưa tải được",
    FILTERS_LOAD_ERROR_DESCRIPTION:
      "Bạn vẫn có thể xem và tìm kiếm bài tập. Hãy thử lại để dùng đầy đủ bộ lọc độ khó và chủ đề.",
    RETRY: "Tải lại",
    GO_TO_PAGE: (page: number) => `Đi đến trang ${page}`,
    CURRENT_PAGE: (page: number) => `Trang hiện tại, trang ${page}`,
    PREVIOUS_PAGE_LABEL: "Đi đến trang trước",
    NEXT_PAGE_LABEL: "Đi đến trang sau",
    SEARCH_PLACEHOLDER: "Tìm kiếm danh tự bài tập...",
    SEARCH_LABEL: "Tìm bài tập theo tên",
    OPEN_PROBLEM: (title: string) => `Mở bài tập ${title}`,
    SOLVED_LABEL: "Đã giải bài này",
    UNSOLVED_LABEL: "Chưa giải bài này",
    SOLVE_STATUS_UNAVAILABLE: "Đăng nhập để xem trạng thái đã giải",
    ACCEPTANCE_VALUE: (value: string) => `Tỷ lệ chấp nhận ${value}`,
    SUBMISSION_VALUE: (count: string) => `${count} lượt nộp`,
    TABLE_LABEL: "Danh sách bài tập luyện tập",
    // Filter Panel
    FILTER_BUTTON: "Lọc & Sắp Xếp",
    FILTER_TITLE: "Bộ Lọc & Sắp Xếp",
    FILTER_DESCRIPTION:
      "Lọc bài tập theo độ khó, chủ đề và sắp xếp theo kết quả chấm.",
    SORT_TITLE: "Sắp Xếp Tiên Bảng",
    SORT_ACCEPTANCE: "Tỷ Lệ AC",
    SORT_SOLVED: "Lượt Nộp",
    DIFFICULTY_TITLE: "Phẩm Chất",
    TAGS_TITLE: "Công Pháp",
    CLEAR_FILTER: "Làm mới bộ lọc",
    ACTIVE_FILTERS: "Đang lọc:",
    CLEAR_ALL: "Xóa tất cả",
    NO_DIFFICULTIES: "Chưa có độ khó nào được công bố.",
    NO_TAGS: "Chưa có chủ đề nào được công bố.",
    TAG_LIMIT_REACHED: (maximum: number) =>
      `Bạn có thể chọn tối đa ${maximum} chủ đề cùng lúc.`,
    ASC: "Tăng dần",
    DESC: "Giảm dần",
    DEFAULT: "Mặc định",
  },

  // Contest
  CONTEST: {
    TITLE: "Đại Hội Tỷ Thí",
    SUBTITLE: "Các giải đấu lập trình định kỳ — thi đấu, leo rank, tranh danh hiệu.",
    SEARCH_LABEL: "Tìm kiếm cuộc thi theo tên",
    SEARCH_PLACEHOLDER: "Tìm kiếm cuộc thi...",
    LIST_LOADING: "Đang tải danh sách cuộc thi...",
    REFRESHING: "Đang cập nhật danh sách cuộc thi",
    LOAD_ERROR_TITLE: "Không thể tải danh sách cuộc thi",
    LOAD_ERROR_DESCRIPTION:
      "Kết nối dữ liệu cuộc thi đang gián đoạn. Hãy thử tải lại.",
    RETRY: "Thử tải lại",
    NO_CONTESTS: "Chưa có cuộc thi nào",
    NO_CONTESTS_DESC: "Hiện chưa có đại hội nào được tổ chức.",
    NO_MATCHING_CONTESTS_DESC:
      "Không có cuộc thi phù hợp với bộ lọc hiện tại.",
    REGISTER: "Đăng Ký Tham Gia",
    LOGIN_TO_REGISTER: "Đăng nhập để đăng ký",
    UNREGISTER: "Hủy Đăng Ký",
    REGISTERED: "Đã Đăng Ký",
    REGISTER_SUCCESS: "Đăng ký thành công!",
    REGISTER_ERROR: "Đăng ký thất bại",
    UNREGISTER_SUCCESS: "Đã hủy đăng ký.",
    UNREGISTER_ERROR: "Hủy đăng ký thất bại",
    PARTICIPANTS: "Người Tham Gia",
    START_TIME: "Bắt Đầu",
    END_TIME: "Kết Thúc",
    STATUS: "Trạng Thái",
    STANDINGS: "Bảng Xếp Hạng",
    RANK: "Hạng",
    USER: "Đại Năng",
    SOLVED: "AC",
    PENALTY: "Phạt",
    PROBLEMS: "Bài",
    MAX_PARTICIPANTS: "Giới Hạn",
    // Status labels
    STATUS_DRAFT: "Nháp",
    STATUS_UPCOMING: "Sắp Diễn Ra",
    STATUS_RUNNING: "Đang Diễn Ra",
    STATUS_ENDED: "Đã Kết Thúc",
    STATUS_CANCELLED: "Đã Hủy",
    // Detail
    BACK_TO_LIST: "Đại Hội Tỷ Thí",
    DETAIL_DESCRIPTION: "Mô Tả",
    DETAIL_INFO: "Thông Tin",
    DETAIL_TABS_LABEL: "Nội dung cuộc thi",
    DETAIT_STANDINGS: "Bảng Xếp Hạng",
    DETAIL_LOADING: "Đang tải dữ liệu cuộc thi...",
    // Rating changes
    RATING_CHANGES: "Biến Động Rating",
    RATING_OLD: "Rating Cũ",
    RATING_NEW: "Rating Mới",
    RATING_DELTA: "Thay Đổi",
    NO_RATING_CHANGES: "Chưa có dữ liệu rating",
    NO_RATING_CHANGES_DESC: "Rating sẽ được tính sau khi cuộc thi kết thúc.",
    RATING_LOADING: "Đang tải biến động rating...",
    RATING_LOAD_ERROR: "Không thể tải biến động rating",
    RATING_LOAD_ERROR_DESCRIPTION:
      "Dữ liệu hiện chưa khả dụng. Hãy thử tải lại để nhận kết quả mới nhất.",
    FILTER_ALL: "Tất Cả",
    FILTER_RUNNING: "Đang Diễn Ra",
    FILTER_UPCOMING: "Sắp Tới",
    FILTER_ENDED: "Đã Kết Thúc",
    NOT_FOUND: "Không tìm thấy cuộc thi",
    NOT_FOUND_DESCRIPTION:
      "Cuộc thi không tồn tại, chưa được công bố hoặc đường dẫn không hợp lệ.",
    DETAIL_LOAD_ERROR_TITLE: "Không thể tải cuộc thi",
    DETAIL_LOAD_ERROR_DESCRIPTION:
      "Dữ liệu cuộc thi hiện chưa khả dụng. Hãy thử tải lại sau ít phút.",
    NO_DESCRIPTION: "Chưa có mô tả.",
    PROBLEM_NUMBER: (label: string | number) => `Bài ${label}`,
    PROBLEM_ID: (id: string) => `ID: ${id}`,
    PROBLEM_POINTS: (points: number) => `${points} điểm`,
    NO_PROBLEMS: "Chưa có bài tập.",
    VIEW_RESULTS: "XEM KẾT QUẢ",
    VIEW_DETAILS: "XEM ĐẠI HỘI",
    JOIN_NOW: "THAM GIA NGAY",
    REGISTER_EARLY: "ĐĂNG KÝ TRƯỚC",
    HERO_IMAGE_ALT: "Khán đài đại hội lập trình",
    HERO_BADGE: "ĐẠI HỘI QUẦN HÙNG",
    HERO_DESCRIPTION:
      "Nơi vạn tông hội tụ, thiên tài tranh phong. Khai mở bí cảnh, đoạt lấy thiên địa linh bảo và ghi danh trên Vạn Cổ Tiên Bảng.",
    HERO_REGISTER: "KHÁM PHÁ ĐẠI HỘI",
    HERO_SCHEDULE: "XEM LỊCH SẮP TỚI",
    STANDINGS_EMPTY: "Chưa có dữ liệu xếp hạng",
    STANDINGS_EMPTY_DESCRIPTION:
      "Bảng xếp hạng sẽ hiển thị khi cuộc thi bắt đầu.",
    STANDINGS_LOADING: "Đang tải bảng xếp hạng...",
    STANDINGS_LOAD_ERROR: "Không thể tải bảng xếp hạng",
    STANDINGS_LOAD_ERROR_DESCRIPTION:
      "Kết nối dữ liệu xếp hạng đang gián đoạn. Hãy thử tải lại.",
    STANDINGS_REFRESH_ERROR:
      "Chưa thể làm mới; đang hiển thị snapshot gần nhất.",
    STANDINGS_FROZEN_LABEL: "Bảng xếp hạng đang đóng băng",
    STANDINGS_FROZEN_DESCRIPTION: (time: string) =>
      `Kết quả từ ${time} trở đi được giữ kín đến khi cuộc thi kết thúc. Số bài đang chờ vẫn được hiển thị nhưng không tiết lộ verdict.`,
    STANDINGS_PENDING: "Đang chờ",
    STANDINGS_PENDING_COUNT: (count: number) =>
      `${count} lượt nộp sau thời điểm đóng băng`,
    STANDINGS_MY_POSITION: "Vị Trí Chính Thức Của Bạn",
    STANDINGS_MY_POSITION_DESCRIPTION:
      "Chỉ bạn thấy projection này; bảng công khai vẫn giữ đúng mốc đóng băng.",
    STANDINGS_MY_POSITION_ERROR:
      "Chưa thể tải vị trí riêng của bạn. Bảng công khai bên dưới vẫn chính xác.",
    CONTENT_TAB: "Thông báo & làm rõ",
    ANNOUNCEMENTS: "Thông báo",
    CLARIFICATIONS: "Làm rõ",
    CONTENT_LOADING: "Đang tải nội dung cuộc thi...",
    CONTENT_ERROR: "Không thể tải nội dung cuộc thi. Hãy thử lại.",
    CONTENT_REFRESH_ERROR:
      "Chưa thể đồng bộ dữ liệu mới; nội dung gần nhất vẫn được giữ lại.",
    CONTENT_REFRESH: "Đồng bộ",
    CONTENT_REFRESHING: "Đang đồng bộ...",
    CONTENT_LOAD_MORE: "Xem thêm",
    CONTENT_LOADING_MORE: "Đang tải thêm...",
    CONTENT_END: "Đã hiển thị toàn bộ",
    NO_ANNOUNCEMENTS: "Chưa có thông báo.",
    NO_CLARIFICATIONS: "Chưa có câu hỏi làm rõ được công bố.",
    NO_OWN_CLARIFICATIONS: "Bạn chưa gửi câu hỏi làm rõ nào.",
    MY_QUESTIONS: "Câu hỏi của tôi",
    ASK_QUESTION: "Gửi câu hỏi làm rõ",
    QUESTION_LABEL: "Nội dung câu hỏi",
    QUESTION_PLACEHOLDER: "Mô tả rõ vấn đề bạn cần ban giám khảo hỗ trợ...",
    SEND_QUESTION: "Gửi câu hỏi",
    SENDING_QUESTION: "Đang gửi...",
    QUESTION_SENT: "Đã gửi câu hỏi làm rõ.",
    QUESTION_SEND_ERROR: "Không thể gửi câu hỏi. Hãy thử lại.",
    QUESTION_INPUT_INVALID:
      "Câu hỏi không được để trống hoặc vượt quá giới hạn dung lượng.",
    QUESTION_WITHDRAW: "Rút câu hỏi",
    QUESTION_WITHDRAW_TITLE: "Rút câu hỏi làm rõ",
    QUESTION_WITHDRAW_DESCRIPTION:
      "Câu hỏi sẽ không còn được xử lý. Thao tác này được lưu vào nhật ký và không thể hoàn tác.",
    QUESTION_WITHDRAW_REASON_LABEL: "Lý do rút câu hỏi",
    QUESTION_WITHDRAW_REASON_PLACEHOLDER:
      "Cho ban giám khảo biết vì sao câu hỏi không còn cần thiết...",
    QUESTION_WITHDRAWING: "Đang rút...",
    QUESTION_WITHDRAW_SUCCESS: "Đã rút câu hỏi làm rõ.",
    QUESTION_WITHDRAW_ERROR:
      "Không thể rút câu hỏi. Dữ liệu có thể vừa thay đổi, hãy đồng bộ và thử lại.",
    QUESTION_CREATED_AT: (value: string) => `Gửi lúc ${value}`,
    ANNOUNCEMENT_PUBLISHED_AT: (value: string) => `Công bố ${value}`,
    ANSWER: "Trả lời",
    SUMMARY: "Tóm tắt câu hỏi đã được duyệt",
    PRIVATE: "Riêng tư",
    SHARED_WITH_PARTICIPANTS: "Hiển thị cho người tham gia",
    STATUS_OPEN: "Đang chờ trả lời",
    STATUS_ANSWERED: "Đã trả lời",
    STATUS_CLOSED: "Đã đóng",
    STATUS_WITHDRAWN: "Đã rút",
    JURY_INBOX: "Hộp thư ban giám khảo",
    JURY_FILTER_ALL: "Tất cả",
    JURY_FILTER_OPEN: "Chờ xử lý",
    JURY_DETAIL: "Chi tiết làm rõ",
    JURY_ANSWER: "Trả lời câu hỏi",
    JURY_ANSWER_LABEL: "Nội dung trả lời",
    JURY_SUMMARY_LABEL: "Tóm tắt an toàn cho người tham gia",
    JURY_AUDIENCE_LABEL: "Phạm vi trả lời",
    JURY_AUDIENCE_REQUESTER: "Chỉ người hỏi",
    JURY_AUDIENCE_PARTICIPANTS: "Người tham gia",
    JURY_REASON_LABEL: "Lý do thao tác",
    JURY_CLOSE: "Đóng yêu cầu",
    JURY_HISTORY: "Lịch sử phiên bản",
    JURY_NO_HISTORY: "Chưa có phiên bản nào.",
    JURY_LOAD_ERROR: "Không thể tải hộp thư ban giám khảo.",
    JURY_SAVE_SUCCESS: "Đã cập nhật câu trả lời.",
    JURY_SAVE_ERROR: "Không thể cập nhật câu trả lời.",
    JURY_CLOSE_SUCCESS: "Đã đóng yêu cầu làm rõ.",
    JURY_CLOSE_ERROR: "Không thể đóng yêu cầu làm rõ.",
    COMMUNICATIONS_TITLE: "Trung tâm điều hành cuộc thi",
    COMMUNICATIONS_SUBTITLE:
      "Soạn thông báo, quản lý lịch công bố và xử lý câu hỏi của thí sinh trong một không gian làm việc.",
    COMMUNICATIONS_ANNOUNCEMENTS_TAB: "Thông báo cuộc thi",
    COMMUNICATIONS_CLARIFICATIONS_TAB: "Hộp thư làm rõ",
    ANNOUNCEMENT_CREATE: "Soạn thông báo",
    ANNOUNCEMENT_EDIT: "Chỉnh sửa bản nháp",
    ANNOUNCEMENT_EDITOR_DESCRIPTION:
      "Nội dung được render và làm sạch ở máy chủ trước khi có thể công bố.",
    ANNOUNCEMENT_TITLE_LABEL: "Tiêu đề",
    ANNOUNCEMENT_TITLE_PLACEHOLDER: "Thông tin quan trọng dành cho thí sinh",
    ANNOUNCEMENT_CONTENT_LABEL: "Nội dung Markdown",
    ANNOUNCEMENT_CONTENT_PLACEHOLDER:
      "Viết nội dung rõ ràng, ngắn gọn và đầy đủ ngữ cảnh...",
    ANNOUNCEMENT_AUDIENCE_LABEL: "Đối tượng nhận",
    ANNOUNCEMENT_AUDIENCE_PUBLIC: "Công khai",
    ANNOUNCEMENT_AUDIENCE_PARTICIPANTS: "Thí sinh đã đăng ký",
    ANNOUNCEMENT_AUDIENCE_JURY: "Ban giám khảo",
    ANNOUNCEMENT_SCHEDULE_LABEL: "Thời điểm công bố tự động",
    ANNOUNCEMENT_SCHEDULE_HELP:
      "Để trống nếu muốn lưu bản nháp và công bố thủ công.",
    ANNOUNCEMENT_REASON_LABEL: "Lý do thay đổi",
    ANNOUNCEMENT_REASON_PLACEHOLDER:
      "Ghi rõ mục đích để lưu vào nhật ký kiểm toán...",
    ANNOUNCEMENT_SAVE_DRAFT: "Lưu bản nháp",
    ANNOUNCEMENT_SAVE_CHANGES: "Lưu thay đổi",
    ANNOUNCEMENT_SAVING: "Đang lưu...",
    ANNOUNCEMENT_CANCEL_EDIT: "Hủy chỉnh sửa",
    ANNOUNCEMENT_CREATE_SUCCESS: "Đã tạo bản nháp thông báo.",
    ANNOUNCEMENT_UPDATE_SUCCESS: "Đã cập nhật bản nháp thông báo.",
    ANNOUNCEMENT_SAVE_ERROR: "Không thể lưu thông báo.",
    ANNOUNCEMENT_LIST_TITLE: "Danh sách thông báo",
    ANNOUNCEMENT_LIST_EMPTY: "Chưa có thông báo phù hợp với bộ lọc.",
    ANNOUNCEMENT_LIST_ERROR: "Không thể tải danh sách thông báo.",
    ANNOUNCEMENT_FILTER_ALL: "Tất cả trạng thái",
    ANNOUNCEMENT_FILTER_DRAFT: "Bản nháp",
    ANNOUNCEMENT_FILTER_PUBLISHED: "Đã công bố",
    ANNOUNCEMENT_FILTER_WITHDRAWN: "Đã thu hồi",
    ANNOUNCEMENT_STATUS_DRAFT: "Bản nháp",
    ANNOUNCEMENT_STATUS_PUBLISHED: "Đã công bố",
    ANNOUNCEMENT_STATUS_WITHDRAWN: "Đã thu hồi",
    ANNOUNCEMENT_EDIT_ACTION: "Chỉnh sửa",
    ANNOUNCEMENT_PUBLISH_ACTION: "Công bố",
    ANNOUNCEMENT_PUBLISH_UPDATE_ACTION: "Công bố bản cập nhật",
    ANNOUNCEMENT_WITHDRAW_ACTION: "Thu hồi",
    ANNOUNCEMENT_ACTION_REASON_TITLE: "Xác nhận thao tác",
    ANNOUNCEMENT_ACTION_REASON_DESCRIPTION:
      "Thao tác này được ghi vào nhật ký kiểm toán và cần một lý do cụ thể.",
    ANNOUNCEMENT_PUBLISH_SUCCESS: "Đã công bố thông báo.",
    ANNOUNCEMENT_WITHDRAW_SUCCESS: "Đã thu hồi thông báo.",
    ANNOUNCEMENT_ACTION_ERROR: "Không thể cập nhật trạng thái thông báo.",
    ANNOUNCEMENT_SCHEDULED_FOR: (value: string) => `Hẹn công bố ${value}`,
    ANNOUNCEMENT_UPDATED_AT: (value: string) => `Cập nhật ${value}`,
    ANNOUNCEMENT_REVISION: (value: number) => `Phiên bản nội dung ${value}`,
    ANNOUNCEMENT_HISTORY_DESCRIPTION:
      "Các phiên bản trước là bất biến và được giữ lại để kiểm toán.",
    JURY_INBOX_EMPTY: "Không có câu hỏi nào trong trạng thái này.",
    JURY_SELECT_PROMPT:
      "Chọn một câu hỏi ở danh sách bên trái để xem nội dung và phản hồi.",
    JURY_QUESTION_LABEL: "Câu hỏi của thí sinh",
    JURY_QUESTION_CREATED_AT: (value: string) => `Gửi lúc ${value}`,
    JURY_LOAD_PREVIOUS: "Trang trước",
    JURY_LOAD_NEXT: "Trang tiếp",
    JURY_PAGE: (value: number) => `Trang ${value}`,
    JURY_REFRESH_STALE:
      "Dữ liệu vừa thay đổi. Đã tải lại phiên bản mới nhất để bạn tiếp tục.",
    JURY_HISTORY_ERROR: "Không thể tải lịch sử phiên bản.",
    JURY_HISTORY_KIND_QUESTION: "Câu hỏi",
    JURY_HISTORY_KIND_ANNOUNCEMENT: "Nội dung thông báo",
    JURY_HISTORY_KIND_SUMMARY: "Tóm tắt công khai",
    JURY_HISTORY_KIND_ANSWER: "Câu trả lời",
    JURY_STATUS_LABEL: "Trạng thái yêu cầu",
    FORM_BYTES_USED: (used: number, maximum: number) =>
      `${used}/${maximum} byte`,
  },

  // Problem Detail
  PROBLEM: {
    BACK_TO_ARENA: "Luyện Tập",
    BACK_TO_CONTEST: "Quay Lại Đại Hội",
    DESCRIPTION: "Đề Bài",
    EXAMPLES: "Ví Dụ",
    EXAMPLE: "Ví dụ",
    INPUT: "Dữ liệu vào",
    OUTPUT: "Kết quả mẫu",
    EXPLANATION: "Giải thích",
    COPIED: "Đã sao chép",
    COPY: "Sao chép",
    SUBMISSION_HISTORY: "Lịch Sử Nộp Bài",
    NO_SUBMISSIONS: "Chưa có lần nộp bài nào",
    NO_SUBMISSIONS_DESCRIPTION:
      "Lần nộp tiếp theo sẽ xuất hiện tại đây và được cập nhật khi judge hoàn tất.",
    HISTORY_AUTH_REQUIRED_TITLE: "Đăng nhập để xem lịch sử",
    HISTORY_AUTH_REQUIRED_DESCRIPTION:
      "Lịch sử và kết quả chấm được bảo vệ theo tài khoản của bạn.",
    HISTORY_AUTH_REQUIRED_ACTION: "Đăng nhập",
    HISTORY_LOAD_ERROR_TITLE: "Chưa thể tải lịch sử nộp bài",
    HISTORY_LOAD_ERROR_DESCRIPTION:
      "Source code của bạn vẫn an toàn. Hãy thử đồng bộ lại mà không cần tải lại trang.",
    HISTORY_REFRESH: "Đồng bộ",
    HISTORY_REFRESHING: "Đang đồng bộ...",
    HISTORY_LOAD_MORE: "Tải thêm bài nộp",
    HISTORY_LOADING_MORE: "Đang tải thêm...",
    HISTORY_END: "Đã hiển thị toàn bộ lịch sử",
    HISTORY_LIVE: "Cập nhật trực tiếp",
    HISTORY_CONNECTING: "Đang kết nối judge",
    HISTORY_FALLBACK: "Đồng bộ dự phòng",
    HISTORY_VIEW_DETAIL: "Xem chi tiết bài nộp",
    HISTORY_TABLE_LABEL: "Lịch sử nộp bài của bạn",
    HISTORY_LATEST_RESULT: (status: string) =>
      `Kết quả bài nộp mới nhất: ${status}`,
    HISTORY_STATUS_LABEL: "Trạng thái đồng bộ kết quả",
    HISTORY_OPEN_SUBMISSION: (id: string) =>
      `Xem chi tiết bài nộp ${id}`,
    SUBMIT: "Nộp Bài",
    SUBMITTING: "Đang gửi...",
    WORKBENCH_TITLE: "Không gian làm bài",
    WORKBENCH_DESCRIPTION:
      "Viết trực tiếp hoặc nhập source code từ máy của bạn.",
    AUTH_REQUIRED_TO_SUBMIT:
      "Đăng nhập để nộp bài và theo dõi kết quả chấm theo thời gian thực.",
    SESSION_CHECKING: "Đang xác minh phiên đăng nhập...",
    LOGIN_TO_SUBMIT: "Đăng nhập để nộp bài",
    JUMP_TO_WORKBENCH: "Đi tới trình soạn thảo",
    JUMP_TO_STATEMENT: "Quay lại đề bài",
    PROBLEM_MOBILE_NAVIGATION: "Điều hướng không gian làm bài",
    SELECT_LANGUAGE: "Ngôn ngữ",
    SELECT_FILE: "Chọn file code",
    IMPORT_SOURCE: "Nhập file",
    CHECKING_FILE: "Đang kiểm tra file...",
    FILE_SELECTED: "Đã chọn",
    NO_FILE: "Chưa chọn file",
    NOT_FOUND: "Không tìm thấy bài tập",
    NOT_FOUND_DESCRIPTION:
      "Bài tập không tồn tại, chưa được xuất bản hoặc đã bị gỡ.",
    LOADING: "Đang tải nội dung bài tập...",
    LOAD_ERROR_TITLE: "Chưa thể tải bài tập",
    LOAD_ERROR_DESCRIPTION:
      "Kết nối tới dữ liệu bài tập đang bị gián đoạn. Bạn có thể thử lại mà không mất phiên đăng nhập.",
    RETRY: "Thử tải lại",
    SAMPLES_LOADING: "Đang tải ví dụ kiểm thử...",
    STATEMENT_RENDERING: "Đang hiển thị nội dung đề bài...",
    SAMPLES_LOAD_ERROR_TITLE: "Chưa thể tải ví dụ kiểm thử",
    SAMPLES_LOAD_ERROR_DESCRIPTION:
      "Nội dung đề bài vẫn sử dụng được. Hãy thử lại để xem dữ liệu vào và kết quả mẫu.",
    SAMPLES_EMPTY: "Bài tập này chưa có ví dụ công khai.",
    SUBMIT_SUCCESS: "Đã chấp nhận! Chúc mừng đạo hữu đã giải thành công!",
    TIME_LIMIT: "Thời gian",
    MEMORY_LIMIT: "Bộ nhớ",
    ACCEPTANCE_RATE_LABEL: (value: string) =>
      `Tỷ lệ chấp nhận ${value}`,
    SUBMISSION_COUNT_LABEL: (value: string) =>
      `${value} lượt nộp`,
    INPUT_FORMAT: "Định dạng đầu vào",
    OUTPUT_FORMAT: "Định dạng đầu ra",
    // Submission history table
    LOADING_SUBMISSIONS: "Đang tải lịch sử nộp bài...",
    COL_RESULT: "Kết Quả",
    COL_LANGUAGE: "Ngôn Ngữ",
    COL_TESTS: "Kiểm thử",
    COL_TIME: "Thời Gian",
    COL_MEMORY: "Bộ Nhớ",
    COL_SUBMITTED_AT: "Nộp Lúc",
    // Submission detail sheet
    DETAIL_TITLE: "Chi Tiết Nộp Bài",
    DETAIL_LOADING: "Đang tải chi tiết bài nộp...",
    DETAIL_LOAD_ERROR: "Không thể tải chi tiết bài nộp.",
    DETAIL_RETRY: "Thử lại",
    DETAIL_SOURCE_CODE: "Mã nguồn",
    DETAIL_ERROR: "Thông Báo Lỗi",
    DETAIL_STATS_TESTS: "Kiểm thử đạt",
    DETAIL_STATS_TIME: "Thời gian",
    DETAIL_STATS_MEMORY: "Bộ nhớ",
    DETAIL_COPY: "Sao chép",
    DETAIL_COPIED: "Đã sao chép",
    DETAIL_COPY_ERROR: "Không thể sao chép source code.",
    DETAIL_SUBMISSION_ID: "Mã bài nộp",
    SUBMIT_SUCCESS_SHORT: "Nộp bài thành công!",
    SUBMIT_ERROR: "Nộp bài thất bại, thử lại sau.",
    CONTEST_CONTEXT_INVALID:
      "Liên kết bài thi đã cũ hoặc không đầy đủ. Hãy mở lại bài từ trang cuộc thi.",
    CONTEST_CONTEXT_INVALID_TITLE: "Ngữ cảnh cuộc thi không hợp lệ",
    DIFFICULTY_UNAVAILABLE: "N/A",
    STATUS_COLUMN: "Trạng Thái",
    TIME_UNIT_MS: "ms",
    MEMORY_UNIT_MB: "MB",
    CLEAR_FILE: "Bỏ tệp đã chọn",
    CLEAR_SOURCE: "Xóa source",
    EDITOR_LABEL: "Trình soạn thảo source code",
    EDITOR_PLACEHOLDER: (filename: string) =>
      `Viết lời giải cho ${filename} tại đây...`,
    SOURCE_MANUAL: "Soạn thảo trực tiếp",
    SOURCE_IMPORTED: (filename: string) => `Đã nhập ${filename}`,
    SOURCE_CODE_REQUIRED: "Source code không được để trống.",
    SOURCE_FILE_UNSUPPORTED: (extensions: string) =>
      `Định dạng file chưa được hỗ trợ. Hãy dùng: ${extensions}.`,
    SOURCE_FILE_TOO_LARGE: (maximum: string) =>
      `File code vượt quá giới hạn ${maximum}.`,
    SOURCE_FILE_INVALID_UTF8: "File code phải được mã hoá UTF-8 hợp lệ.",
    SOURCE_FILE_CONTAINS_NUL: "File code không được chứa byte NUL.",
    SOURCE_FILE_REQUIRED: "File code không được để trống.",
    SOURCE_FILE_READ_ERROR: "Không thể đọc file code. Vui lòng chọn lại.",
    SOURCE_FILE_INVALID: "Nội dung file code không hợp lệ.",
    SOURCE_FILE_SIZE: (actual: string, maximum: string) =>
      `${actual} / ${maximum}`,
    RUNTIME_CATALOG_LOADING: "Đang đồng bộ môi trường chấm bài...",
    RUNTIME_CATALOG_ERROR_TITLE: "Chưa thể tải môi trường chấm bài",
    RUNTIME_CATALOG_ERROR_DESCRIPTION:
      "Danh sách ngôn ngữ được lấy trực tiếp từ judge để tránh nộp source vào runtime không được hỗ trợ.",
    RUNTIME_NOT_ALLOWED_TITLE: "Chưa có runtime tương thích",
    RUNTIME_NOT_ALLOWED_DESCRIPTION:
      "Revision này không còn runtime tương thích với release judge đang hoạt động. Quản trị viên cần kiểm tra lại release trước khi nhận bài nộp.",
    RUNTIME_READY: "Judge sẵn sàng",
    RUNTIME_FILE: "Tệp nguồn",
    RUNTIME_COMPILED: "Biên dịch",
    RUNTIME_INTERPRETED: "Thông dịch",
    SUBMIT_SHORTCUT: "Ctrl/⌘ + Enter để nộp",
    INDENT_WITH_TAB: "Dùng Tab để thụt dòng",
    INDENT_WITH_TAB_HINT:
      "Tắt để phím Tab chuyển tiêu điểm sang điều khiển tiếp theo.",
    EDITOR_ESCAPE_HINT:
      "Nhấn Escape để tắt chế độ thụt dòng và rời trình soạn thảo bằng Tab.",
    COPY_INPUT: "Sao chép dữ liệu vào",
    COPY_OUTPUT: "Sao chép kết quả mẫu",
    COPY_FAILED: "Không thể sao chép nội dung.",
    SUBMISSION_STATUS: {
      ACCEPTED: "Đã chấp nhận",
      WRONG_ANSWER: "Sai đáp án",
      TIME_LIMIT: "Quá thời gian",
      MEMORY_LIMIT: "Quá bộ nhớ",
      PROCESS_LIMIT: "Vượt giới hạn tiến trình",
      OUTPUT_LIMIT: "Vượt giới hạn đầu ra",
      RUNTIME_ERROR: "Lỗi khi chạy",
      COMPILE_ERROR: "Lỗi biên dịch",
      PENDING: "Đang chờ",
      JUDGING: "Đang chấm...",
      INTERNAL_ERROR: "Lỗi hệ thống",
    },
  },

  MATERIALS: {
    NOT_FOUND: "Không tìm thấy bài viết",
    BACK_TO_LIBRARY: "Quay lại Tàng Kinh Các",
    LIBRARY: "Tàng Kinh Các",
    ARTICLE: "Bài viết",
    READ_MINUTES: (minutes: number) => `${minutes} phút đọc`,
    VIEW_COUNT: (views: number) => `${views} lượt xem`,
    TOGGLE_CATEGORIES: "Mở hoặc đóng danh mục",
    CATEGORY_MENU: "Danh mục tài liệu",
    ALL_ARTICLES: "Tất Cả Bài Viết",
    EMPTY: "Không tìm thấy bài viết nào",
    EMPTY_DESCRIPTION: "Thử thay đổi bộ lọc hoặc từ khóa tìm kiếm",
    ARTICLES_LOADING: "Đang tải bài viết...",
    ARTICLE_LOADING: "Đang tải nội dung bài viết...",
    ARTICLES_REFRESHING: "Đang cập nhật bài viết...",
    ARTICLES_LOAD_MORE: "Xem thêm bài viết",
    ARTICLES_LOADING_MORE: "Đang tải thêm...",
    ARTICLES_END: "Đã hiển thị toàn bộ bài viết.",
    ARTICLES_LOAD_ERROR_TITLE: "Không thể tải bài viết",
    ARTICLES_LOAD_ERROR_DESCRIPTION:
      "Danh sách tài liệu hiện chưa khả dụng. Vui lòng thử lại.",
    ARTICLE_LOAD_ERROR_TITLE: "Không thể tải bài viết",
    ARTICLE_LOAD_ERROR_DESCRIPTION:
      "Nội dung bài viết hiện chưa khả dụng. Vui lòng thử lại.",
    CATEGORIES_LOADING: "Đang tải danh mục...",
    CATEGORIES_LOAD_ERROR_TITLE: "Không thể tải danh mục",
    CATEGORIES_LOAD_ERROR_DESCRIPTION:
      "Danh mục tạm thời chưa khả dụng. Tìm kiếm và bộ lọc độ khó vẫn hoạt động.",
    DIFFICULTIES_LOADING: "Đang tải độ khó...",
    DIFFICULTIES_LOAD_ERROR_TITLE: "Không thể tải độ khó",
    DIFFICULTIES_LOAD_ERROR_DESCRIPTION:
      "Bộ lọc độ khó tạm thời chưa khả dụng. Danh mục và tìm kiếm vẫn hoạt động.",
    SEARCH_LABEL: "Tìm kiếm bài viết",
    DIFFICULTY_FILTER_LABEL: "Lọc theo độ khó",
    SEARCH_UPDATING: "Đang chờ cập nhật tìm kiếm...",
    RETRY: "Thử lại",
    CATEGORY_TITLE: "Danh Mục",
    ALL: "Tất Cả",
    DIFFICULTY_BEGINNER: "Nhập Môn",
    DIFFICULTY_BASIC: "Cơ Bản",
    DIFFICULTY_ADVANCED: "Nâng Cao",
    HERO_DESCRIPTION:
      "Kho tài liệu học thuật từ cơ bản đến nâng cao, bao gồm cấu trúc dữ liệu, giải thuật, ngôn ngữ lập trình và thiết kế hệ thống.",
    SEARCH_PLACEHOLDER: "Tìm kiếm bài viết...",
    ARTICLE_COUNT_LABEL: "bài viết",
    MINUTES_SHORT: "phút",
    RELATED_TITLE: "Tài Liệu Liên Quan",
    RELATED_LOADING: "Đang tải tài liệu liên quan...",
    RELATED_LOAD_ERROR_TITLE: "Không thể tải tài liệu liên quan",
    RELATED_LOAD_ERROR_DESCRIPTION:
      "Bài viết chính vẫn khả dụng. Bạn có thể thử tải lại phần gợi ý.",
    RELATED_EMPTY: "Chưa có tài liệu liên quan phù hợp.",
  },

  CULTIVATION: {
    TRAIT_CODEX: "Xem tất cả traits",
    EXP_MULTIPLIER: "EXP ×",
    EXP: "EXP",
    TRAINING_SPEED: "Tu Luyện Tốc",
    BASE_EXP: "EXP Cơ Bản",
    CODEX_TITLE: "Thiên Mệnh Thư Quán",
    CODEX_DESCRIPTION:
      "Tổng hợp Căn Cốt và Thiên Phú trong thế giới tu luyện",
    CODEX_LOADING: "Đang tải thiên mệnh thư quán...",
    CODEX_LOAD_ERROR: "Không thể tải thiên mệnh thư quán",
    CODEX_LOAD_ERROR_DESCRIPTION:
      "Danh mục Căn Cốt và Thiên Phú đang tạm thời không khả dụng. Vui lòng thử lại.",
    CODEX_STALE:
      "Đang hiển thị danh mục gần nhất vì lần đồng bộ vừa rồi chưa thành công.",
    CODEX_ROOT_EMPTY: "Chưa có Căn Cốt nào trong danh mục.",
    CODEX_TALENT_EMPTY: "Chưa có Thiên Phú nào trong danh mục.",
    NEED_MORE: "Cần thêm",
    DEFAULT_RARITY: "Phàm Phẩm",
    ELEMENT_FIRE: "Hỏa",
    ELEMENT_FIRE_SHORT: "H",
    ELEMENT_WATER: "Thủy",
    ELEMENT_WATER_SHORT: "T",
    ELEMENT_METAL: "Kim",
    ELEMENT_METAL_SHORT: "K",
    ELEMENT_WOOD: "Mộc",
    ELEMENT_WOOD_SHORT: "M",
    ELEMENT_EARTH: "Thổ",
    ELEMENT_EARTH_SHORT: "Đ",
  },

  // Ranking
  RANKING: {
    TAB_RATING: "Thiên Đạo Bảng",
    TAB_LEVEL: "Tu Luyện Bảng",
    TAB_SPIRITUAL_ROOT: "Linh Căn Bảng",
    RATING_SUBTITLE: "Xếp hạng theo cảnh giới tu luyện — cấp bậc đại đạo",
    LEVEL_SUBTITLE: "Xếp hạng theo cấp độ và kinh nghiệm tu luyện",
    SPIRITUAL_ROOT_SUBTITLE: "Xếp hạng theo từng hệ linh căn",
    LEVEL: "Cấp Độ",
    EXP: "Kinh Nghiệm",
    PURITY: "Độ Thuần Khiết",
    SPIRITUAL_ROOT: "Linh Căn",
    REALM: "Cảnh Giới",
    SECT: "Tông Môn",
    POINTS: "Linh Lực",
    HERO_BADGE: "VẠN CỔ TIÊN BẢNG",
    HERO_DESC: "Nơi vinh danh những bậc đại năng có căn cốt phi phàm, khắc tên vào sử sách của giới tu chân.",
    TABLE_TITLE: "Quần Anh Hội Tụ",
    RANK_LABEL: "Thứ Hạng",
    CULTIVATOR_LABEL: "Vị Đại Năng",
    RATING_SECTION: "Thiên Đạo Bảng",
    CULTIVATION_SECTION: "Tu Luyện Bảng",
    RATING_SECTION_SUBTITLE: "Cảnh giới tu luyện",
    CULTIVATION_SECTION_SUBTITLE: "Cấp độ & danh hiệu",
    HERO_IMAGE_ALT: "Celestial Heaven",
    EMPTY_CULTIVATORS: "Chưa có tu sĩ",
    LEVEL_ABBREVIATION: "Lv.",
    SPIRIT_POWER: "Linh Lực",
    POINTS_ABBREVIATION: "Pts",
    LOADING_LABEL: "Đang tải bảng xếp hạng",
    LOAD_ERROR_TITLE: "Không thể tải bảng xếp hạng",
    LOAD_ERROR_DESCRIPTION:
      "Dữ liệu xếp hạng đang tạm thời không khả dụng. Vui lòng thử lại sau ít phút.",
    EMPTY_DESCRIPTION:
      "Bảng sẽ tự động hiển thị khi có thành tích xếp hạng đầu tiên.",
    STALE_DATA:
      "Đang hiển thị dữ liệu gần nhất vì lần làm mới vừa rồi chưa thành công.",
	REFRESHING: "Đang làm mới bảng xếp hạng",
	ANONYMOUS: "Tu sĩ ẩn danh",
	LOAD_MORE: "Xem thêm thứ hạng",
	LOADING_MORE: "Đang tải thêm...",
	END_OF_LIST: "Đã hiển thị toàn bộ thứ hạng hiện có.",
  },

  // Theme
  THEME: {
    LIGHT: "Sáng",
    DARK: "Tối",
    SYSTEM: "Hệ Thống",
    LABEL: "Giao diện",
    SELECT: "Chọn giao diện",
    TOGGLE: "Chuyển đổi giao diện",
  },

  // Common
  COMMON: {
    LOADING: "Đang tải...",
    PAGE_LOADING: "Đang chuẩn bị nội dung trang...",
    WORKSPACE_LOADING: "Đang chuẩn bị không gian làm bài...",
    ADMIN_LOADING: "Đang chuẩn bị không gian quản trị...",
    AUTH_LOADING: "Đang chuẩn bị phiên xác thực...",
    ERROR: "Có lỗi xảy ra",
    RETRY: "Thử lại",
    PAGINATION: "Phân trang",
    SAVE: "Lưu",
    CANCEL: "Hủy",
    DELETE: "Xóa",
    EDIT: "Sửa",
    CREATE: "Tạo Mới",
    SEARCH: "Tìm Kiếm",
    SEARCH_PLACEHOLDER: "Tìm kiếm...",
    FILTER: "Lọc",
    NO_DATA: "Không có dữ liệu",
    UNKNOWN: "Không xác định",
    NOT_AVAILABLE: "Chưa cập nhật",
    UNITS: {
      MILLISECONDS_SHORT: "ms",
      SECONDS_SHORT: "s",
      MINUTES_SHORT: "ph",
      HOURS_SHORT: "g",
      KILOBYTES_SHORT: "KB",
      MEGABYTES_SHORT: "MB",
    },
    PAGE: "Trang",
    OF: "trong số",
    PREVIOUS: "Trước",
    NEXT: "Tiếp",
    GO_TO_PAGE: (page: number) => `Đi đến trang ${page}`,
    CURRENT_PAGE: (page: number) => `Trang hiện tại, trang ${page}`,
    PAGE_POSITION: (current: number, total: number) =>
      `Trang ${current}/${total}`,
    CLOSE: "Đóng",
    CLOSE_NOTIFICATION: "Đóng thông báo",
    NOTIFICATIONS: "Thông báo",
    TOTAL: "Tổng",
    ITEMS: "mục",
    ITEM_COUNT: "{count} mục",
    CONFIRM_DELETE_TITLE: "Xác nhận xoá",
    CONFIRM_DELETE_DESCRIPTION:
      "Bạn có chắc chắn muốn xoá? Hành động này không thể hoàn tác.",
    DELETE_ACCENTED: "Xoá",
    SAVE_CHANGES: "Lưu Thay Đổi",
    CREATE_NEW: "Tạo Mới",
  },
} as const;
