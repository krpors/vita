package main

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoRepository struct {
	userCollection     *mongo.Collection
	cvCollection       *mongo.Collection
	revisionCollection *mongo.Collection
}

func NewMongoRepository(client *mongo.Client) MongoRepository {
	userCollection := client.Database("vita").Collection("user")
	cvCollection := client.Database("vita").Collection("cv")
	revisionCollection := client.Database("vita").Collection("cv_revision")

	return MongoRepository{
		userCollection,
		cvCollection,
		revisionCollection,
	}
}

func (r *MongoRepository) Authenticate(ctx context.Context, username, password string) (User, bool, error) {
	log.Printf("Finding username with name '%s'", username)
	// TODO: can use extra key to find by username AND password
	// TODO: also hash and salt the password before storing it
	cursor, err := r.userCollection.Find(ctx, bson.D{
		{Key: "username", Value: username},
	})
	if err != nil {
		return User{}, false, err
	}

	var user []User
	cursor.All(ctx, &user)
	log.Printf("Amount of users with name '%s': %d", username, len(user))

	if len(user) == 1 {
		if user[0].Password == password {
			return user[0], true, nil
		}

		log.Printf("Unable to authenticate for user '%s': passwords do not match", username)
		return User{}, false, nil
	}

	return User{}, false, nil
}

// Gets the current CV for the given user.
func (r *MongoRepository) GetCurrentCV(ctx context.Context, uid string) (CurriculumVitaeDocument, bool, error) {
	var none CurriculumVitaeDocument
	oid, err := bson.ObjectIDFromHex(uid)
	if err != nil {
		return none, false, err
	}

	log.Printf("Getting current CV for user ID '%s'", oid)

	singleResult := r.cvCollection.FindOne(ctx, bson.D{
		{Key: "metadata.owner", Value: oid},
	})

	if singleResult.Err() == mongo.ErrNoDocuments {
		// Nothing found. See documentation of FindOne
		return none, false, nil
	}

	if singleResult.Err() != nil {
		// Other error
		return none, false, singleResult.Err()
	}

	var cv CurriculumVitaeDocument
	if err := singleResult.Decode(&cv); err != nil {
		return none, false, err
	}
	log.Printf("Retrieved CV: %v", cv.Id)

	return cv, true, nil
}

func (r *MongoRepository) SaveNewCV(ctx context.Context, uid string, cv *CurriculumVitae) error {
	previousCv, err := r.MoveCurrentCVToRevisions(ctx, uid)
	if err != nil {
		return err
	}

	ownerId, err := bson.ObjectIDFromHex(uid)
	if err != nil {
		return err
	}

	newCvDoc := CurriculumVitaeDocument{
		Metadata: Metadata{
			Owner:   ownerId,
			Version: previousCv.Metadata.Version + 1,
		},
		Cv: *cv,
	}

	insertResult, err := r.cvCollection.InsertOne(ctx, newCvDoc)
	if err != nil {
		return err
	}

	log.Printf("Inserted new CV with ID %s", insertResult.InsertedID)

	return nil
}

// MoveCurrentsCVToRevisions moved the (possible) current user's CV to the revisions
// collection.
func (r *MongoRepository) MoveCurrentCVToRevisions(ctx context.Context, uid string) (CurriculumVitaeDocument, error) {
	var none CurriculumVitaeDocument
	previousCv, found, err := r.GetCurrentCV(ctx, uid)

	if err != nil {
		return none, err
	}

	if !found {
		log.Printf("User does not have a CV yet, nothing moved to revision collection")
		return none, nil
	}

	ownerId, err := bson.ObjectIDFromHex(uid)
	if err != nil {
		return none, err
	}

	deleteResult, err := r.cvCollection.DeleteOne(ctx, bson.D{
		{Key: "metadata.owner", Value: ownerId},
	})

	if err != nil {
		return none, err
	}

	log.Printf("Deleted %d CV for user %s", deleteResult.DeletedCount, uid)

	insertResult, err := r.revisionCollection.InsertOne(ctx, previousCv)
	if err != nil {
		return none, err
	}
	log.Printf("Inserted CV with ID %s into revisions", insertResult.InsertedID)
	return previousCv, nil
}

func (r *MongoRepository) FindRevisionsForUser(ctx context.Context, uid string) ([]CurriculumVitaeDocument, error) {
	var revisionDocs []CurriculumVitaeDocument

	oid, err := bson.ObjectIDFromHex(uid)
	if err != nil {
		return nil, err
	}

	cursor, err := r.revisionCollection.Find(ctx, bson.D{
		{Key: "metadata.owner", Value: oid},
	})

	if err != nil {
		return nil, nil
	}

	if err = cursor.All(ctx, &revisionDocs); err != nil {
		return nil, err
	}

	log.Printf("Found %d revisions for user %s", len(revisionDocs), uid)

	return revisionDocs, nil
}

func (r *MongoRepository) DeleteAllRevisions(ctx context.Context, uid string) (int64, error) {
	oid, err := bson.ObjectIDFromHex(uid)
	if err != nil {
		return 0, err
	}

	deleteResult, err := r.revisionCollection.DeleteMany(ctx, bson.D{
		{Key: "metadata.owner", Value: oid},
	})

	if err != nil {
		return 0, err
	}

	log.Printf("Deleted %d revision CVs assigned to user %s", deleteResult.DeletedCount, uid)

	return deleteResult.DeletedCount, nil
}

func (r *MongoRepository) DeleteSingleRevision(ctx context.Context, uid string, version int) (bool, error) {
	oid, err := bson.ObjectIDFromHex(uid)
	if err != nil {
		return false, err
	}

	deleteResult, err := r.revisionCollection.DeleteOne(ctx, bson.D{
		{Key: "metadata.owner", Value: oid},
		{Key: "metadata.version", Value: version},
	})

	if err != nil {
		return false, err
	}

	return deleteResult.DeletedCount > 0, nil
}

func (r *MongoRepository) FindAllUsers(ctx context.Context) {
	cursor, err := r.userCollection.Find(ctx, bson.D{})
	if err != nil {
		panic(err)
	}

	var user []User
	cursor.All(ctx, &user)

	for _, v := range user {
		_ = v
		// fmt.Printf("User: %s %s\n", v.Profile.FirstName, v.Profile.LastName)
	}
}

func (r *MongoRepository) CreateUser(ctx context.Context, user User) (bson.ObjectID, error) {
	log.Printf("checking for user %v", user)
	result := r.userCollection.FindOne(ctx, bson.D{
		{Key: "username", Value: user.Username},
	})

	if result.Err() == nil {
		// no error, user found
		return bson.NilObjectID, fmt.Errorf("user already exists with name '%s'", user.Username)
	}

	if result.Err() == mongo.ErrNoDocuments {
		// user does not exist yet, so we can create it.

		res, err := r.userCollection.InsertOne(ctx, user)
		if err != nil {
			return bson.NilObjectID, err
		}
		log.Printf("User created with name '%s' and ID '%s'", user.Username, res.InsertedID)
		return res.InsertedID.(bson.ObjectID), nil
	}

	return bson.NilObjectID, result.Err()
}

// DeleteUser deletes a user from the Mongo database, plus any resource assigned
// to it, such as revisions, cvs etc. This method should obviously only be called
// in special cases, such as by administrators, or the (logged in) user themselves.
func (r *MongoRepository) DeleteUser(ctx context.Context, uid string) error {
	objectId, err := bson.ObjectIDFromHex(uid)
	if err != nil {
		return err
	}

	result, err := r.cvCollection.DeleteMany(ctx, bson.D{
		{Key: "metadata.owner", Value: objectId},
	})
	if err != nil {
		log.Printf("Could not delete CV for user '%s': %s", uid, err)
		return err
	}

	log.Printf("Deleted %d CVs assigned to user %s", result.DeletedCount, uid)

	_, err = r.DeleteAllRevisions(ctx, uid)
	if err != nil {
		log.Printf("Could not delete revisions for user '%s': %s", uid, err)
		return err
	}

	result, err = r.userCollection.DeleteMany(ctx, bson.D{
		{Key: "_id", Value: objectId},
	})
	if err != nil {
		log.Printf("Could not delete user '%s': %s", uid, err)
		return err
	}

	log.Printf("Deleted %d user from the database", result.DeletedCount)
	return nil
}
