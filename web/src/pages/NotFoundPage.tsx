import { Link } from "react-router-dom";

export default function NotFoundPage() {
  return (
    <section>
      <h1 className="text-2xl font-semibold">Page not found</h1>
      <p className="mt-2 text-gray-600">
        The page you’re looking for doesn’t exist.{" "}
        <Link to="/" className="text-red-700 hover:underline">
          Back to dashboard
        </Link>
        .
      </p>
    </section>
  );
}
