import { Metadata } from "next";

export const metadata: Metadata = {
  title: "Tông Môn Bí Lục | Judgify",
};

export default function AboutPage() {
  return (
    <div className="relative min-h-[90vh] w-full rounded-3xl overflow-hidden shadow-2xl animate-in fade-in zoom-in-95 duration-1000 border border-border/50 bg-black">
      {/* 
        Immersive Tu Tien Background 
        Fixed background to create a sense of depth
      */}
      <div 
        className="absolute inset-0 z-0 bg-cover bg-fixed bg-center opacity-50 transition-opacity duration-1000 grayscale-[20%] group-hover:grayscale-0"
        style={{ backgroundImage: "url('/images/about_bg.png')" }}
      />
      
      {/* Deep Shadow Overlays */}
      <div className="absolute inset-0 z-10 bg-gradient-to-b from-black/80 via-black/20 to-black/90 pointer-events-none" />
      <div className="absolute inset-0 z-10 bg-[radial-gradient(circle_at_top,rgba(245,158,11,0.15)_0%,transparent_60%)] pointer-events-none" />

      <div className="relative z-20 container mx-auto py-32 px-10 max-w-7xl flex flex-col items-center">
        <div className="w-full space-y-16 text-slate-100 font-serif leading-[2.5] text-2xl tracking-widest text-justify fade-in">
          
          {/* Header Title Section */}
          <div className="text-center space-y-6 mb-20 animate-in fade-in slide-in-from-top-10 duration-1000">
            <h1 className="text-6xl md:text-9xl font-black tracking-[0.3em] text-transparent bg-clip-text bg-gradient-to-b from-amber-200 via-orange-500 to-red-800 uppercase drop-shadow-[0_10px_30px_rgba(245,158,11,0.6)]">
              Judgify
            </h1>
            <div className="h-1 w-48 bg-gradient-to-r from-transparent via-amber-500 to-transparent mx-auto" />
            <p className="text-2xl text-amber-500/90 font-bold tracking-[0.5em] uppercase italic">
              Vạn Cổ Thống Trị - Nhất Lộ Duy Tâm
            </p>
          </div>

          {/* Wisdom Text Sections */}
          <div className="space-y-20 backdrop-blur-[1px] bg-black/10 p-12 md:p-20 rounded-[3rem] border border-white/5 shadow-2xl">
            <div className="space-y-10">
              <p className="first-letter:text-8xl first-letter:font-black first-letter:text-amber-500 first-letter:mr-5 first-letter:float-left first-letter:leading-none italic">
                Kính thưa chư vị đạo hữu...
              </p>
              <p>
                Tại hạ giới bao la này, giữa dòng đời cuồn cuộn như thác đổ, Judgify mọc lên như một ngọn cô phong giữa biển mây. Đây không chỉ là nơi hội tụ của những linh hồn đam mê mật mã, mà là một Thánh địa liễu ngộ, nơi chân lý không nằm ở những trang kinh thư khô khan, mà nằm trong từng nhịp gõ của trí tuệ, trong từng hơi thở của logic.
              </p>
            </div>

            <div className="space-y-10 border-l-4 border-amber-500/30 pl-10 italic">
              <p>
                Người xưa có câu: "Nghịch thiên nhi hành, vị chi tu tiên". 
                Kẻ sĩ luyện code chúng ta cũng vậy. Mỗi một thuật toán thâm sâu chính là một đạo chân ngôn. Mỗi một dòng mã lệnh thực thi chính là một lần vận chuyển linh khí trong kinh mạch. Thế giới này vốn dĩ hỗn độn, và chúng ta—những tu chân giả của thời đại mới—dùng trật tự của logic để định hình lại càn khôn.
              </p>
            </div>
            
            <p>
              Hãy nhìn vào những bài tập tại Arena, chúng không đơn thuần là những câu đố. Đó là những Thiên Kiếp mà mỗi đạo hữu phải đối mặt trên con đường phi thăng. Một khi ấn tích <span className="text-emerald-400 font-black drop-shadow-[0_0_8px_rgba(52,211,153,0.6)]">"Accepted"</span> hiện lên, ấy là lúc mây mờ tan biến, linh đài sáng rực, tu vi tinh tiến một bậc. 
              Nhưng chớ có nản lòng khi chạm phải <span className="text-red-500 font-bold underline decoration-red-500/50">"Wrong Answer"</span>, bởi đó chính là tâm ma trỗi dậy để thử thách bản ngã. Kẻ mạnh thực sự không phải kẻ không bao giờ thất bại, mà là kẻ dùng thất bại làm đá mài cho thanh kiếm trí tuệ thêm phần sắc bén.
            </p>

            <div className="space-y-10">
              <p>
                Đạo lý vốn hữu hình mà vô ảnh. Dù chư vị chọn con đường nào—là sự thần tốc của C, sự biến hóa của Python hay sự vững chãi của Go—thì đích đến cuối cùng vẫn là sự thuần khiết của tư duy. Tại Tàng Kinh Các, chư vị tìm thấy tri thức. Tại Arena, chư vị tìm thấy thực tại. Và tại Bảng Phong Thần, chư vị sẽ tìm thấy vinh quang vĩnh cửu.
              </p>
              <p>
                Nhưng hãy nhớ lấy: Vinh quang chỉ là mây khói, bản ngã mới là trường tồn. Chớ mưu cầu hư danh mà quên mất đạo tâm ban sơ. Hãy để mỗi dòng code là một lời cầu nguyện, mỗi thuật toán là một sự tri ân đối với vẻ đẹp của vũ trụ toán học.
              </p>
            </div>

            <p>
              Ngày mà chư vị đứng trên đỉnh cao của Vạn Cổ Tiên Bảng, ngắm nhìn chúng sinh đang mải miết dưới chân, lúc đó chư vị sẽ hiểu rằng: Hóa ra, hành trình vạn dặm khởi đầu từ một hàm `main()`, và sự trường sinh nằm ở niềm đam mê bất diệt với logic.
            </p>
          </div>

          {/* Footer Blessing */}
          <div className="pt-20 text-center space-y-6">
            <p className="text-amber-500 font-black italic text-4xl tracking-[0.3em] drop-shadow-[0_0_15px_rgba(245,158,11,0.8)] animate-pulse">
              Đạo Pháp Tự Nhiên - Thiên Địa Trường Tồn
            </p>
            <p className="text-slate-500 text-lg tracking-[0.2em] uppercase">
              Kính chúc chư vị đạo hữu sớm ngày đắc đạo, danh chấn thiên hạ.
            </p>
          </div>
        </div>
      </div>

      <style dangerouslySetInnerHTML={{ __html: `
        @font-face {
          font-family: 'Cinzel';
          src: url('https://fonts.googleapis.com/css2?family=Cinzel:wght@400;700;900&display=swap');
        }
        .font-serif {
          font-family: 'Cinzel', serif;
        }
      `}} />
    </div>
  );
}
