import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { createRequest } from "../lib/requests-api";
import {
  BLOOD_TYPES,
  URGENCIES,
  type BloodType,
  type CreateRequestInput,
  type Urgency,
} from "../types/request";

export default function NewRequestPage() {
  const navigate = useNavigate();

  const [bloodType, setBloodType] = useState<BloodType | "">("");
  const [unitsNeeded, setUnitsNeeded] = useState("1");
  const [urgency, setUrgency] = useState<Urgency>("routine");
  const [radiusM, setRadiusM] = useState("10000");
  const [neededBy, setNeededBy] = useState("");
  const [notes, setNotes] = useState("");

  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!bloodType) {
      setError("Please choose a blood type");
      return;
    }
    setError(null);
    setSubmitting(true);

    const input: CreateRequestInput = {
      blood_type: bloodType,
      units_needed: Number(unitsNeeded),
      urgency,
      radius_m: Number(radiusM),
    };
    if (notes.trim()) {
      input.notes = notes.trim();
    }
    if (neededBy) {
      // datetime-local has no timezone; convert to an RFC3339 UTC timestamp.
      input.needed_by = new Date(neededBy).toISOString();
    }

    try {
      const created = await createRequest(input);
      navigate(`/requests/${created.id}`, { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to create request");
      setSubmitting(false);
    }
  }

  const fieldClass =
    "mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-red-500 focus:outline-none focus:ring-1 focus:ring-red-500";

  return (
    <section className="max-w-lg">
      <h1 className="text-2xl font-semibold">New request</h1>
      <p className="mt-1 text-sm text-gray-500">
        The request starts as pending approval and is matched to donors once a
        clinic admin approves it.
      </p>

      {error && (
        <p
          role="alert"
          className="mt-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700"
        >
          {error}
        </p>
      )}

      <form onSubmit={handleSubmit} className="mt-6 space-y-4">
        <label className="block text-sm font-medium text-gray-700">
          Blood type
          <select
            required
            value={bloodType}
            onChange={(e) => setBloodType(e.target.value as BloodType)}
            className={fieldClass}
          >
            <option value="" disabled>
              Select…
            </option>
            {BLOOD_TYPES.map((bt) => (
              <option key={bt} value={bt}>
                {bt}
              </option>
            ))}
          </select>
        </label>

        <label className="block text-sm font-medium text-gray-700">
          Units needed
          <input
            type="number"
            required
            min={1}
            max={100}
            value={unitsNeeded}
            onChange={(e) => setUnitsNeeded(e.target.value)}
            className={fieldClass}
          />
        </label>

        <label className="block text-sm font-medium text-gray-700">
          Urgency
          <select
            value={urgency}
            onChange={(e) => setUrgency(e.target.value as Urgency)}
            className={`${fieldClass} capitalize`}
          >
            {URGENCIES.map((u) => (
              <option key={u} value={u} className="capitalize">
                {u}
              </option>
            ))}
          </select>
        </label>

        <label className="block text-sm font-medium text-gray-700">
          Search radius (meters)
          <input
            type="number"
            min={1}
            max={100000}
            value={radiusM}
            onChange={(e) => setRadiusM(e.target.value)}
            className={fieldClass}
          />
          <span className="mt-1 block text-xs font-normal text-gray-500">
            Defaults to 10,000 m (10 km). Donors are matched within this radius
            of the clinic.
          </span>
        </label>

        <label className="block text-sm font-medium text-gray-700">
          Needed by <span className="font-normal text-gray-400">(optional)</span>
          <input
            type="datetime-local"
            value={neededBy}
            onChange={(e) => setNeededBy(e.target.value)}
            className={fieldClass}
          />
        </label>

        <label className="block text-sm font-medium text-gray-700">
          Notes <span className="font-normal text-gray-400">(optional)</span>
          <textarea
            rows={3}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            className={fieldClass}
          />
        </label>

        <div className="flex items-center gap-3 pt-2">
          <button
            type="submit"
            disabled={submitting}
            className="rounded-md bg-red-700 px-4 py-2 text-sm font-medium text-white hover:bg-red-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {submitting ? "Creating…" : "Create request"}
          </button>
          <Link
            to="/requests"
            className="text-sm text-gray-600 hover:text-gray-900"
          >
            Cancel
          </Link>
        </div>
      </form>
    </section>
  );
}
