import RegisterFlow from "@/modules/auth/RegisterFlow";
import { AuthSurface } from "@/modules/auth/auth-surface";

export default function RegisterPage() {
  return (
    <AuthSurface className="overflow-hidden" framed>
      <div className="p-5 lg:p-8 relative z-10">
        <RegisterFlow />
      </div>
    </AuthSurface>
  );
}
