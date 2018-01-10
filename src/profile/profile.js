// @flow
import {findProfiles} from './../database/queries';
import {DB} from '../database/db';
import {NoProfilePresent} from '../errors';
import type {PublicProfile} from '../specification/publicProfile.js';
import {ProfileObject} from '../database/schemata';
import type {EthUtilsInterface} from '../ethereum/utils';

export const PROFILE_VERSION = '1.0.0';

export interface Profile {

    hasProfile() : Promise<boolean>;
    setProfile(pseudo: string, description: string, image: string) : Promise<void>;
    getProfile() : Promise<ProfileObject>;
    getPublicProfile(): Promise<PublicProfile>

}

/**
 * Set profile data
 * @param {object} db object that implements the DBInterface
 * @return {function(string, string, string)}
 */
export function setProfile(db: DB): (pseudo: string, description: string, image: string) => Promise<void> {
    return (pseudo: string, description: string, image: string): Promise<void> => {
        return new Promise((res, rej) => {
            db.write((realm: any) => {
                // Since a user can create only one profile
                // we will updated the existing one if it exist

                const profiles:Array<ProfileObject> = findProfiles(realm);

                // Create profile if no exist
                if (profiles.length === 0) {
                    realm.create('Profile', {
                        id: profiles.length +1,
                        pseudo: pseudo,
                        description: description,
                        image: image,
                        version: PROFILE_VERSION,
                    });

                    res();
                    return;
                }

                // Updated existing profile
                const profile = profiles[0];

                profile.pseudo = pseudo;
                profile.description = description;
                profile.image = image;

                res();
            });
        });
    };
}

/**
 *
 * @param {object} db
 * @param {function} query
 * @return {function()}
 */
export function hasProfile(db: DB, query: (realm: any) => Array<{...any}>): (() => Promise<boolean>) {
    'use strict';
    return (): Promise<boolean> => {
        return new Promise((res, rej) => {
            db.query(query)
                .then((profiles) => {
                    if (profiles.length >= 1) {
                        res(true);
                        return;
                    }

                    res(false);
                })
                .catch((e) => rej(e));
        });
    };
}

/**
 * @desc Fetch profile
 * @param {object} db object that implements the DBInterface
 * @param {function} query
 * @return {function()}
 */
export function getProfile(db: DB, query: (realm: any) => Array<{...any}>): (() => Promise<ProfileObject>) {
    return (): Promise<{...any}> => {
        return new Promise((res, rej) => {
            db.query(query)

                // Fetch the first profile or reject if user has no profiles
                .then((profiles) => {
                    if (profiles.length <= 0) {
                        rej(new NoProfilePresent());
                        return;
                    }

                    res(profiles[0]);
                })

                .catch((err) => rej(err));
        });
    };
}

/**
 *
 * @param {object} ethUtils object that implements the EthUtilsInterface
 * @param {function} getProfile function that return's an profile
 * @return {function()}
 */
export function getPublicProfile(ethUtils: {...any}, getProfile: () => Promise<{...any}>): () => Promise<PublicProfile> {
    return (): Promise<PublicProfile> => {
        return new Promise(async function(res, rej) {
            try {
                // Fetch saved profile
                const sp = await getProfile();

                // Public profile
                const pubProfile:PublicProfile = {
                    pseudo: sp.pseudo,
                    description: sp.description,
                    image: sp.image,
                    ethAddresses: [],
                    version: '1.0.0',
                };

                // Fetch all keypairs
                const keyPairs = await ethUtils.allKeyPairs();

                keyPairs.map((keyPair) => {
                    pubProfile.ethAddresses.push(keyPair.key);
                });

                res(pubProfile);
            } catch (e) {
                rej(e);
            }
        });
    };
}

/**
 *
 * @param {object} db object that implements DBInterface
 * @param {object} ethUtils
 * @return {Profile}
 */
export default function(db: DB, ethUtils: EthUtilsInterface): Profile {
    const profileImplementation : Profile = {

        hasProfile: hasProfile(db, findProfiles),

        setProfile: setProfile(db),

        getProfile: getProfile(db, findProfiles),

        getPublicProfile: getPublicProfile(ethUtils, getProfile(db, findProfiles)),

    };

    return profileImplementation;
}
