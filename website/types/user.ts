/**
 * User-related types mirroring backend identity DTOs.
 */

/** User profile returned by GET /users/profile. */
export interface UserProfile {
  username: string;
  first_name: string;
  last_name: string;
  /** 0 = male, 1 = female, 2 = other */
  gender: number;
  /** Format: "2006-01-02" */
  birthday: string;
}

/** PUT /users/profile request body for updating profile. */
export interface UpdateProfileRequest {
  first_name: string;
  last_name: string;
  gender: number;
  birthday: string;
}
