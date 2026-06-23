import { cookies } from "next/headers";
import { redirect } from "next/navigation";

export default async function HomePage() {
  const cookieStore = await cookies();
  const accessToken = cookieStore.get("access_token")?.value;
  const refreshToken = cookieStore.get("refresh_token")?.value;

  // Redirect to dashboard if authenticated, otherwise to login
  if (accessToken || refreshToken) {
    redirect("/dashboard");
  }
  redirect("/login");
}
