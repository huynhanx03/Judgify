import type { MaterialArticle, MaterialCategory } from "@/types/material";

export const MOCK_CATEGORIES: MaterialCategory[] = [
  {
    id: "dsa",
    name: "Cấu Trúc Dữ Liệu & Giải Thuật",
    description: "Nền tảng cốt lõi của lập trình thi đấu",
    iconName: "Network",
    articleCount: 8,
  },
  {
    id: "golang",
    name: "Golang",
    description: "Ngôn ngữ lập trình hiệu năng cao của Google",
    iconName: "Code2",
    articleCount: 5,
  },
  {
    id: "cpp",
    name: "C/C++",
    description: "Ngôn ngữ truyền thống cho lập trình thi đấu",
    iconName: "Terminal",
    articleCount: 4,
  },
  {
    id: "dp",
    name: "Quy Hoạch Động",
    description: "Kỹ thuật tối ưu hóa bài toán con chồng chéo",
    iconName: "Layers",
    articleCount: 6,
  },
  {
    id: "graph",
    name: "Đồ Thị",
    description: "Lý thuyết đồ thị và các thuật toán liên quan",
    iconName: "GitBranch",
    articleCount: 7,
  },
  {
    id: "math",
    name: "Toán Học",
    description: "Số học, tổ hợp, xác suất và hình học tính toán",
    iconName: "Calculator",
    articleCount: 5,
  },
  {
    id: "string",
    name: "Xử Lý Chuỗi",
    description: "Thuật toán trên chuỗi và pattern matching",
    iconName: "TextCursorInput",
    articleCount: 4,
  },
  {
    id: "system-design",
    name: "Thiết Kế Hệ Thống",
    description: "Kiến trúc phần mềm và hệ thống phân tán",
    iconName: "Server",
    articleCount: 3,
  },
];

export const MOCK_ARTICLES: MaterialArticle[] = [
  // DSA
  {
    id: "dsa-1",
    title: "Mảng và Danh Sách Liên Kết",
    description: "Cấu trúc dữ liệu tuyến tính cơ bản, so sánh ưu nhược điểm và ứng dụng thực tế.",
    difficulty: "Nhập Môn",
    tags: ["Array", "Linked List", "Linear"],
    categoryId: "dsa",
    content: `### Giới thiệu
Mảng (Array) và Danh sách liên kết (Linked List) là hai cấu trúc dữ liệu tuyến tính cơ bản nhất trong khoa học máy tính.

### Mảng (Array)
- Lưu trữ các phần tử **liên tiếp** trong bộ nhớ
- Truy cập phần tử theo chỉ số với độ phức tạp **O(1)**
- Chèn/xóa phần tử ở giữa mảng tốn **O(n)**
- Kích thước cố định khi khai báo (static array)

### Danh Sách Liên Kết (Linked List)
- Mỗi node chứa **data** và **con trỏ** tới node tiếp theo
- Chèn/xóa đầu danh sách chỉ tốn **O(1)**
- Truy cập phần tử thứ k tốn **O(n)**
- Có thể mở rộng linh hoạt không cần biết trước kích thước

### So sánh

- **Truy cập ngẫu nhiên**: Mảng O(1) vs Linked List O(n)
- **Chèn/Xóa đầu**: Mảng O(n) vs Linked List O(1)
- **Bộ nhớ**: Mảng liên tục vs Linked List phân tán (tốn thêm con trỏ)
- **Cache-friendly**: Mảng tốt hơn do locality

### Ứng dụng trong Competitive Programming
- Mảng: hầu hết các bài toán cơ bản, sorting, binary search
- Linked List: implement stack, queue, LRU cache
- \`std::vector\` trong C++ kết hợp ưu điểm cả hai`,
  },
  {
    id: "dsa-2",
    title: "Cây Nhị Phân Tìm Kiếm (BST)",
    description: "Cấu trúc cây BST, các phép toán insert, delete, search và cân bằng cây.",
    difficulty: "Cơ Bản",
    tags: ["Tree", "BST", "Binary Tree"],
    categoryId: "dsa",
  },
  {
    id: "dsa-3",
    title: "Segment Tree & BIT",
    description: "Cây phân đoạn và Binary Indexed Tree cho các bài toán truy vấn đoạn.",
    difficulty: "Nâng Cao",
    tags: ["Segment Tree", "BIT", "Range Query"],
    categoryId: "dsa",
  },
  {
    id: "dsa-4",
    title: "Hash Table và Ứng Dụng",
    description: "Bảng băm, xử lý va chạm, và ứng dụng trong competitive programming.",
    difficulty: "Cơ Bản",
    tags: ["Hash", "Map", "Set"],
    categoryId: "dsa",
  },
  {
    id: "dsa-5",
    title: "Heap và Priority Queue",
    description: "Cấu trúc heap, hàng đợi ưu tiên và ứng dụng trong Dijkstra, Huffman coding.",
    difficulty: "Cơ Bản",
    tags: ["Heap", "Priority Queue"],
    categoryId: "dsa",
  },
  {
    id: "dsa-6",
    title: "Disjoint Set Union (DSU)",
    description: "Union-Find với path compression và union by rank cho bài toán kết nối.",
    difficulty: "Nâng Cao",
    tags: ["DSU", "Union Find", "Connected Components"],
    categoryId: "dsa",
  },
  {
    id: "dsa-7",
    title: "Trie - Cây Tiền Tố",
    description: "Cấu trúc Trie cho tìm kiếm chuỗi, autocomplete và xử lý từ điển.",
    difficulty: "Nâng Cao",
    tags: ["Trie", "String", "Prefix"],
    categoryId: "dsa",
  },
  {
    id: "dsa-8",
    title: "Persistent Data Structures",
    description: "Cấu trúc dữ liệu bền vững, persistent segment tree và ứng dụng nâng cao.",
    difficulty: "Chuyên Sâu",
    tags: ["Persistent", "Segment Tree", "Advanced"],
    categoryId: "dsa",
  },
  // Golang
  {
    id: "go-1",
    title: "Go Cơ Bản - Syntax & Types",
    description: "Cú pháp Go, kiểu dữ liệu, biến, hằng số và các cấu trúc điều khiển.",
    difficulty: "Nhập Môn",
    tags: ["Go", "Syntax", "Basics"],
    categoryId: "golang",
  },
  {
    id: "go-2",
    title: "Goroutines & Channels",
    description: "Lập trình đồng thời trong Go với goroutines, channels và patterns phổ biến.",
    difficulty: "Nâng Cao",
    tags: ["Concurrency", "Goroutine", "Channel"],
    categoryId: "golang",
  },
  {
    id: "go-3",
    title: "Interface & Struct Patterns",
    description: "Thiết kế interface, embedding, composition over inheritance trong Go.",
    difficulty: "Cơ Bản",
    tags: ["Interface", "Struct", "OOP"],
    categoryId: "golang",
  },
  {
    id: "go-4",
    title: "Go Modules & Package Design",
    description: "Quản lý dependency, thiết kế package và best practices cho Go projects.",
    difficulty: "Cơ Bản",
    tags: ["Modules", "Package", "Project"],
    categoryId: "golang",
  },
  {
    id: "go-5",
    title: "Performance Profiling trong Go",
    description: "Sử dụng pprof, benchmark và trace để tối ưu hiệu năng ứng dụng Go.",
    difficulty: "Chuyên Sâu",
    tags: ["Performance", "Profiling", "Benchmark"],
    categoryId: "golang",
  },
  // C/C++
  {
    id: "cpp-1",
    title: "C++ cho Competitive Programming",
    description: "Setup môi trường, template code, IO optimization và STL cơ bản.",
    difficulty: "Nhập Môn",
    tags: ["C++", "CP", "Setup"],
    categoryId: "cpp",
  },
  {
    id: "cpp-2",
    title: "STL Containers & Algorithms",
    description: "Vector, set, map, queue và các thuật toán sort, binary search trong STL.",
    difficulty: "Cơ Bản",
    tags: ["STL", "Container", "Algorithm"],
    categoryId: "cpp",
  },
  {
    id: "cpp-3",
    title: "Bitwise Operations & Bitmask",
    description: "Phép toán bit, bitmask DP và ứng dụng trong tối ưu hóa.",
    difficulty: "Nâng Cao",
    tags: ["Bitwise", "Bitmask", "Optimization"],
    categoryId: "cpp",
  },
  {
    id: "cpp-4",
    title: "Template Metaprogramming",
    description: "Lập trình template nâng cao, SFINAE, concepts và compile-time computation.",
    difficulty: "Chuyên Sâu",
    tags: ["Template", "Metaprogramming", "Advanced"],
    categoryId: "cpp",
  },
  // DP
  {
    id: "dp-1",
    title: "Nhập Môn Quy Hoạch Động",
    description: "Khái niệm DP, top-down vs bottom-up, bài toán Fibonacci và Knapsack cơ bản.",
    difficulty: "Nhập Môn",
    tags: ["DP", "Basics", "Knapsack"],
    categoryId: "dp",
    content: `### Quy Hoạch Động là gì?
Quy hoạch động (Dynamic Programming - DP) là kỹ thuật giải bài toán bằng cách **chia nhỏ** thành các bài toán con, lưu kết quả để tránh tính lại.

### Hai Cách Tiếp Cận
- **Top-down (Memoization)**: Đệ quy + lưu bảng, viết tự nhiên theo công thức
- **Bottom-up (Tabulation)**: Dựng bảng từ dưới lên, thường nhanh hơn do không có overhead đệ quy

### Bài Toán Fibonacci
\`F(n) = F(n-1) + F(n-2)\`
- Đệ quy thuần: **O(2^n)** — quá chậm
- DP top-down: **O(n)** với mảng memo
- DP bottom-up: **O(n)** thời gian, **O(1)** bộ nhớ chỉ cần 2 biến

### Knapsack 0/1
- Cho **n** vật phẩm, mỗi vật có trọng lượng và giá trị
- Ba lô chứa tối đa **W** trọng lượng
- Tìm tổng giá trị lớn nhất
- Công thức: \`dp[i][w] = max(dp[i-1][w], dp[i-1][w-wi] + vi)\`

### Khi Nào Dùng DP?
- Bài toán có **bài toán con chồng chéo** (overlapping subproblems)
- Bài toán có **cấu trúc tối ưu con** (optimal substructure)
- Có thể viết **công thức truy hồi** rõ ràng`,
  },
  {
    id: "dp-2",
    title: "DP Trên Đoạn (Interval DP)",
    description: "Quy hoạch động trên đoạn, bài toán nhân ma trận và cắt thanh.",
    difficulty: "Nâng Cao",
    tags: ["Interval DP", "Matrix Chain"],
    categoryId: "dp",
  },
  {
    id: "dp-3",
    title: "DP Bitmask",
    description: "Quy hoạch động kết hợp bitmask cho bài toán hoán vị và tổ hợp.",
    difficulty: "Nâng Cao",
    tags: ["Bitmask", "Permutation", "Subset"],
    categoryId: "dp",
  },
  {
    id: "dp-4",
    title: "DP Trên Cây",
    description: "Quy hoạch động trên cây, rerooting technique và Heavy-Light Decomposition.",
    difficulty: "Chuyên Sâu",
    tags: ["Tree DP", "Rerooting", "HLD"],
    categoryId: "dp",
  },
  {
    id: "dp-5",
    title: "Tối Ưu DP với Convex Hull Trick",
    description: "Convex Hull Trick, Li Chao Tree và Divide & Conquer optimization.",
    difficulty: "Chuyên Sâu",
    tags: ["CHT", "Li Chao", "Optimization"],
    categoryId: "dp",
  },
  {
    id: "dp-6",
    title: "DP Chữ Số (Digit DP)",
    description: "Kỹ thuật DP chữ số cho bài toán đếm trong khoảng số.",
    difficulty: "Nâng Cao",
    tags: ["Digit DP", "Counting"],
    categoryId: "dp",
  },
  // Graph
  {
    id: "graph-1",
    title: "BFS & DFS Cơ Bản",
    description: "Duyệt đồ thị theo chiều rộng và chiều sâu, ứng dụng tìm đường đi.",
    difficulty: "Nhập Môn",
    tags: ["BFS", "DFS", "Traversal"],
    categoryId: "graph",
  },
  {
    id: "graph-2",
    title: "Đường Đi Ngắn Nhất",
    description: "Dijkstra, Bellman-Ford, Floyd-Warshall và ứng dụng thực tế.",
    difficulty: "Cơ Bản",
    tags: ["Dijkstra", "Bellman-Ford", "Shortest Path"],
    categoryId: "graph",
  },
  {
    id: "graph-3",
    title: "Cây Khung Nhỏ Nhất (MST)",
    description: "Thuật toán Kruskal, Prim và ứng dụng của MST.",
    difficulty: "Cơ Bản",
    tags: ["MST", "Kruskal", "Prim"],
    categoryId: "graph",
  },
  {
    id: "graph-4",
    title: "Luồng Cực Đại (Max Flow)",
    description: "Ford-Fulkerson, Dinic's algorithm và bài toán ghép cặp.",
    difficulty: "Chuyên Sâu",
    tags: ["Max Flow", "Dinic", "Matching"],
    categoryId: "graph",
  },
  {
    id: "graph-5",
    title: "Thành Phần Liên Thông Mạnh",
    description: "Tarjan's algorithm, Kosaraju và ứng dụng SCC trong đồ thị có hướng.",
    difficulty: "Nâng Cao",
    tags: ["SCC", "Tarjan", "Kosaraju"],
    categoryId: "graph",
  },
  {
    id: "graph-6",
    title: "Lowest Common Ancestor (LCA)",
    description: "LCA bằng binary lifting, Euler tour và ứng dụng trên cây.",
    difficulty: "Nâng Cao",
    tags: ["LCA", "Binary Lifting", "Tree"],
    categoryId: "graph",
  },
  {
    id: "graph-7",
    title: "Centroid Decomposition",
    description: "Phân rã centroid trên cây, ứng dụng xử lý truy vấn đường đi.",
    difficulty: "Chuyên Sâu",
    tags: ["Centroid", "Decomposition", "Tree"],
    categoryId: "graph",
  },
  // Math
  {
    id: "math-1",
    title: "Số Học Cơ Bản",
    description: "GCD, LCM, số nguyên tố, sàng Eratosthenes và phân tích thừa số.",
    difficulty: "Nhập Môn",
    tags: ["Number Theory", "Prime", "GCD"],
    categoryId: "math",
  },
  {
    id: "math-2",
    title: "Modular Arithmetic",
    description: "Phép toán modulo, nghịch đảo modular, Fermat nhỏ và CRT.",
    difficulty: "Cơ Bản",
    tags: ["Modular", "Fermat", "CRT"],
    categoryId: "math",
  },
  {
    id: "math-3",
    title: "Tổ Hợp & Xác Suất",
    description: "Nguyên lý đếm, hoán vị, tổ hợp, bao hàm loại trừ.",
    difficulty: "Cơ Bản",
    tags: ["Combinatorics", "Probability", "Counting"],
    categoryId: "math",
  },
  {
    id: "math-4",
    title: "Hình Học Tính Toán",
    description: "Convex hull, line intersection, polygon area và sweep line.",
    difficulty: "Nâng Cao",
    tags: ["Geometry", "Convex Hull", "Sweep Line"],
    categoryId: "math",
  },
  {
    id: "math-5",
    title: "FFT & NTT",
    description: "Fast Fourier Transform, Number Theoretic Transform cho nhân đa thức.",
    difficulty: "Chuyên Sâu",
    tags: ["FFT", "NTT", "Polynomial"],
    categoryId: "math",
  },
  // String
  {
    id: "str-1",
    title: "KMP & Pattern Matching",
    description: "Thuật toán Knuth-Morris-Pratt và các bài toán tìm kiếm chuỗi con.",
    difficulty: "Cơ Bản",
    tags: ["KMP", "Pattern Matching"],
    categoryId: "string",
  },
  {
    id: "str-2",
    title: "String Hashing",
    description: "Rolling hash, Rabin-Karp và ứng dụng so sánh chuỗi O(1).",
    difficulty: "Cơ Bản",
    tags: ["Hashing", "Rabin-Karp"],
    categoryId: "string",
  },
  {
    id: "str-3",
    title: "Suffix Array & LCP",
    description: "Mảng hậu tố, LCP array và ứng dụng tìm chuỗi con chung dài nhất.",
    difficulty: "Nâng Cao",
    tags: ["Suffix Array", "LCP"],
    categoryId: "string",
  },
  {
    id: "str-4",
    title: "Aho-Corasick Automaton",
    description: "Automaton đa pattern matching, ứng dụng tìm nhiều chuỗi đồng thời.",
    difficulty: "Chuyên Sâu",
    tags: ["Aho-Corasick", "Automaton", "Multi-pattern"],
    categoryId: "string",
  },
  // System Design
  {
    id: "sd-1",
    title: "Thiết Kế URL Shortener",
    description: "System design cho dịch vụ rút gọn URL với high availability.",
    difficulty: "Cơ Bản",
    tags: ["URL Shortener", "Hashing", "Database"],
    categoryId: "system-design",
  },
  {
    id: "sd-2",
    title: "Thiết Kế Chat System",
    description: "Real-time messaging system với WebSocket, message queue và storage.",
    difficulty: "Nâng Cao",
    tags: ["Chat", "WebSocket", "Message Queue"],
    categoryId: "system-design",
  },
  {
    id: "sd-3",
    title: "Thiết Kế Rate Limiter",
    description: "Rate limiting algorithms: Token Bucket, Sliding Window và distributed rate limiting.",
    difficulty: "Nâng Cao",
    tags: ["Rate Limiter", "Token Bucket", "Distributed"],
    categoryId: "system-design",
  },
];
