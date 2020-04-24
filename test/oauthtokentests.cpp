#include "gtest/gtest.h"
#include "OAuthToken.h"
#include <time.h>
#include <sys/times.h>

#define CHECK_STRINGS(LEFTSTRING, RIGHTSTRING) ASSERT_EQ(0, LEFTSTRING.compare(RIGHTSTRING)) << "Strings failed equality comparison.  " << #LEFTSTRING << " is " << LEFTSTRING << ", " << #RIGHTSTRING << " is " << RIGHTSTRING << ".  "

// test class to check for OAuthToken fromJson parsing and ensure dates are captured.
namespace Tests
{
	class OAuthTokenTest : public ::testing::Test {
	 public : 
	};

	// Test from json with all the parameters.
	TEST_F(OAuthTokenTest, fromJSonAllParam)
	{ 
		OAuthToken t;
		OAuthToken et = OAuthToken();
		et.access_token = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSIsImtpZCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSJ9.eyJhdWQiOiJodHRwczovL3N0b3JhZ2UuYXp1cmUuY29tLyIsImlzcyI6Imh0dHBzOi8vc3RzLndpbmRvd3MubmV0LzcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0Ny8iLCJpYXQiOjE1ODY4MjE0NTEsIm5iZiI6MTU4NjgyMTQ1MSwiZXhwIjoxNTg2OTA4MTUxLCJhaW8iOiI0MmRnWURnWk1zSERvc3JZV1hyQ01uYnRsTm9UQUE9PSIsImFwcGlkIjoiYzg4MjI5Y2UtYzVlNC00OGJmLWI2NDktYzQ5MGM2ZThmMzQzIiwiYXBwaWRhY3IiOiIyIiwiaWRwIjoiaHR0cHM6Ly9zdHMud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3LyIsIm9pZCI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInN1YiI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInRpZCI6IjcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0NyIsInV0aSI6IjJORU5mZ2dzVTBxZ0JKMC1pQkZuQUEiLCJ2ZXIiOiIxLjAiLCJ4bXNfbWlyaWQiOiIvc3Vic2NyaXB0aW9ucy9iYTQ1YjIzMy1lMmVmLTQxNjktODgwOC00OWViMGQ4ZWJhMGQvcmVzb3VyY2Vncm91cHMvYmxvYmZ1c2UtcmcvcHJvdmlkZXJzL01pY3Jvc29mdC5Db21wdXRlL3ZpcnR1YWxNYWNoaW5lcy9uYXJhZGV2MTYwNCJ9.kBfu0O7sfiYXyRX-kBboyZ1MVM3bdWwKXGQh9N4BR6-xv7ut7HzaEatErRz_BfRDoQEMSwtaNAtrBG8rqWnQNgkAJxaN4vwtdipreOJVMF_94f4BQnxrsjCwuaiGsrZBWS3au2s2CayWSeHEc2vYE8edQBOY58FujLt6h5A1ePssE1TotveazLVesJ3bWgIfH2gedjiy8MKoPWB5GxstcrvuvzDXlG4lFDkUlI8LZ8s0Su8S7KzGPOdkb_1eAGhJdGtDSWI68FGYQsse8OYw0a-d-B06yW3i2NRlLE3_oCy4m-vBKtF2TtpJ5S1eYVa4SkDsl1sVLbJO7E_T4i4gpg";
		//et.client_id = "c88229ce-c5e4-48bf-b649-c490c6e8f343";
		et.expires_in = 86399;
		et.not_before = 1586821451;
		et.resource = "https://storage.azure.com/";
		et.token_type = "Bearer";	
		time_t current_time;
		// get the current time		
		time(&current_time);
		
		//Get GMT time 
		
		struct tm *info;
	   
		info = gmtime(&current_time );
		
		et.expires_on = mktime(info) + 86399;

		std::string json_request_result = "{\"access_token\":\"eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSIsImtpZCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSJ9.eyJhdWQiOiJodHRwczovL3N0b3JhZ2UuYXp1cmUuY29tLyIsImlzcyI6Imh0dHBzOi8vc3RzLndpbmRvd3MubmV0LzcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0Ny8iLCJpYXQiOjE1ODY4MjE0NTEsIm5iZiI6MTU4NjgyMTQ1MSwiZXhwIjoxNTg2OTA4MTUxLCJhaW8iOiI0MmRnWURnWk1zSERvc3JZV1hyQ01uYnRsTm9UQUE9PSIsImFwcGlkIjoiYzg4MjI5Y2UtYzVlNC00OGJmLWI2NDktYzQ5MGM2ZThmMzQzIiwiYXBwaWRhY3IiOiIyIiwiaWRwIjoiaHR0cHM6Ly9zdHMud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3LyIsIm9pZCI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInN1YiI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInRpZCI6IjcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0NyIsInV0aSI6IjJORU5mZ2dzVTBxZ0JKMC1pQkZuQUEiLCJ2ZXIiOiIxLjAiLCJ4bXNfbWlyaWQiOiIvc3Vic2NyaXB0aW9ucy9iYTQ1YjIzMy1lMmVmLTQxNjktODgwOC00OWViMGQ4ZWJhMGQvcmVzb3VyY2Vncm91cHMvYmxvYmZ1c2UtcmcvcHJvdmlkZXJzL01pY3Jvc29mdC5Db21wdXRlL3ZpcnR1YWxNYWNoaW5lcy9uYXJhZGV2MTYwNCJ9.kBfu0O7sfiYXyRX-kBboyZ1MVM3bdWwKXGQh9N4BR6-xv7ut7HzaEatErRz_BfRDoQEMSwtaNAtrBG8rqWnQNgkAJxaN4vwtdipreOJVMF_94f4BQnxrsjCwuaiGsrZBWS3au2s2CayWSeHEc2vYE8edQBOY58FujLt6h5A1ePssE1TotveazLVesJ3bWgIfH2gedjiy8MKoPWB5GxstcrvuvzDXlG4lFDkUlI8LZ8s0Su8S7KzGPOdkb_1eAGhJdGtDSWI68FGYQsse8OYw0a-d-B06yW3i2NRlLE3_oCy4m-vBKtF2TtpJ5S1eYVa4SkDsl1sVLbJO7E_T4i4gpg\",\"client_id\":\"c88229ce-c5e4-48bf-b649-c490c6e8f343\",\"expires_in\":\"86399\",\"expires_on\":\"1586908151\",\"ext_expires_in\":\"86399\",\"not_before\":\"1586821451\",\"resource\":\"https://storage.azure.com/\",\"token_type\":\"Bearer\"}";
		json j;
		j = json::parse(json_request_result);
		from_json(j, t);
		
		fprintf(stdout, "expected auth token expiry time: %s", ctime(&et.expires_on));
		fprintf(stdout, "actual auth token expiry time: %s", ctime(&t.expires_on));
		ASSERT_EQ(t.expires_on, et.expires_on);
		ASSERT_EQ(t.access_token, et.access_token);	
		ASSERT_EQ(t.expires_in, et.expires_in);	
		ASSERT_EQ(t.resource, et.resource);

	}

	// no expires in
	TEST_F(OAuthTokenTest, fromJSonexpOn)
	{ 
		time_t  exp = (time_t) 1586908151;
		OAuthToken t;
		OAuthToken et = OAuthToken();
		et.access_token = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSIsImtpZCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSJ9.eyJhdWQiOiJodHRwczovL3N0b3JhZ2UuYXp1cmUuY29tLyIsImlzcyI6Imh0dHBzOi8vc3RzLndpbmRvd3MubmV0LzcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0Ny8iLCJpYXQiOjE1ODY4MjE0NTEsIm5iZiI6MTU4NjgyMTQ1MSwiZXhwIjoxNTg2OTA4MTUxLCJhaW8iOiI0MmRnWURnWk1zSERvc3JZV1hyQ01uYnRsTm9UQUE9PSIsImFwcGlkIjoiYzg4MjI5Y2UtYzVlNC00OGJmLWI2NDktYzQ5MGM2ZThmMzQzIiwiYXBwaWRhY3IiOiIyIiwiaWRwIjoiaHR0cHM6Ly9zdHMud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3LyIsIm9pZCI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInN1YiI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInRpZCI6IjcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0NyIsInV0aSI6IjJORU5mZ2dzVTBxZ0JKMC1pQkZuQUEiLCJ2ZXIiOiIxLjAiLCJ4bXNfbWlyaWQiOiIvc3Vic2NyaXB0aW9ucy9iYTQ1YjIzMy1lMmVmLTQxNjktODgwOC00OWViMGQ4ZWJhMGQvcmVzb3VyY2Vncm91cHMvYmxvYmZ1c2UtcmcvcHJvdmlkZXJzL01pY3Jvc29mdC5Db21wdXRlL3ZpcnR1YWxNYWNoaW5lcy9uYXJhZGV2MTYwNCJ9.kBfu0O7sfiYXyRX-kBboyZ1MVM3bdWwKXGQh9N4BR6-xv7ut7HzaEatErRz_BfRDoQEMSwtaNAtrBG8rqWnQNgkAJxaN4vwtdipreOJVMF_94f4BQnxrsjCwuaiGsrZBWS3au2s2CayWSeHEc2vYE8edQBOY58FujLt6h5A1ePssE1TotveazLVesJ3bWgIfH2gedjiy8MKoPWB5GxstcrvuvzDXlG4lFDkUlI8LZ8s0Su8S7KzGPOdkb_1eAGhJdGtDSWI68FGYQsse8OYw0a-d-B06yW3i2NRlLE3_oCy4m-vBKtF2TtpJ5S1eYVa4SkDsl1sVLbJO7E_T4i4gpg";
		//et.client_id = "c88229ce-c5e4-48bf-b649-c490c6e8f343";
		et.resource = "https://storage.azure.com/";
		et.token_type = "Bearer";	
		et.expires_on = exp;

		std::string json_request_result = "{\"access_token\":\"eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSIsImtpZCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSJ9.eyJhdWQiOiJodHRwczovL3N0b3JhZ2UuYXp1cmUuY29tLyIsImlzcyI6Imh0dHBzOi8vc3RzLndpbmRvd3MubmV0LzcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0Ny8iLCJpYXQiOjE1ODY4MjE0NTEsIm5iZiI6MTU4NjgyMTQ1MSwiZXhwIjoxNTg2OTA4MTUxLCJhaW8iOiI0MmRnWURnWk1zSERvc3JZV1hyQ01uYnRsTm9UQUE9PSIsImFwcGlkIjoiYzg4MjI5Y2UtYzVlNC00OGJmLWI2NDktYzQ5MGM2ZThmMzQzIiwiYXBwaWRhY3IiOiIyIiwiaWRwIjoiaHR0cHM6Ly9zdHMud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3LyIsIm9pZCI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInN1YiI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInRpZCI6IjcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0NyIsInV0aSI6IjJORU5mZ2dzVTBxZ0JKMC1pQkZuQUEiLCJ2ZXIiOiIxLjAiLCJ4bXNfbWlyaWQiOiIvc3Vic2NyaXB0aW9ucy9iYTQ1YjIzMy1lMmVmLTQxNjktODgwOC00OWViMGQ4ZWJhMGQvcmVzb3VyY2Vncm91cHMvYmxvYmZ1c2UtcmcvcHJvdmlkZXJzL01pY3Jvc29mdC5Db21wdXRlL3ZpcnR1YWxNYWNoaW5lcy9uYXJhZGV2MTYwNCJ9.kBfu0O7sfiYXyRX-kBboyZ1MVM3bdWwKXGQh9N4BR6-xv7ut7HzaEatErRz_BfRDoQEMSwtaNAtrBG8rqWnQNgkAJxaN4vwtdipreOJVMF_94f4BQnxrsjCwuaiGsrZBWS3au2s2CayWSeHEc2vYE8edQBOY58FujLt6h5A1ePssE1TotveazLVesJ3bWgIfH2gedjiy8MKoPWB5GxstcrvuvzDXlG4lFDkUlI8LZ8s0Su8S7KzGPOdkb_1eAGhJdGtDSWI68FGYQsse8OYw0a-d-B06yW3i2NRlLE3_oCy4m-vBKtF2TtpJ5S1eYVa4SkDsl1sVLbJO7E_T4i4gpg\",\"client_id\":\"c88229ce-c5e4-48bf-b649-c490c6e8f343\",\"expires_on\":\"1586908151\",\"not_before\":\"1586821451\",\"resource\":\"https://storage.azure.com/\",\"token_type\":\"Bearer\"}";
		json j;
		j = json::parse(json_request_result);
		from_json(j, t);
		
		fprintf(stdout, "expected auth token expiry time: %s", ctime(&et.expires_on));
		fprintf(stdout, "actual auth token expiry time: %s", ctime(&t.expires_on));
		ASSERT_EQ(t.expires_on, et.expires_on);
		ASSERT_EQ(t.access_token, et.access_token);	
		ASSERT_EQ(t.resource, et.resource);

	}
	 
	// even if there is no expires_on but there is an expires_in it should work
	TEST_F(OAuthTokenTest, fromJSonexpIn)
	{ 
		OAuthToken t;
		OAuthToken et = OAuthToken();
		et.access_token = "eyJ0eXAiOikIjoiYzg4";
		et.resource = "https://storage.azure.com/";
		et.token_type = "Bearer";
		et.expires_in = 86399;
		time_t current_time;
		// get the current time		
		time(&current_time);
		
		//Get GMT time 
		
		struct tm *info;
	   
		info = gmtime(&current_time );
		
		et.expires_on = mktime(info) + 86399;

		std::string json_request_result = "{\"access_token\":\"eyJ0eXAiOikIjoiYzg4\",\"expires_in\":\"86399\",\"ext_expires_in\":\"86399\",\"resource\":\"https://storage.azure.com/\",\"token_type\":\"Bearer\"}";
		json j;
		j = json::parse(json_request_result);
		from_json(j, t);
		
		fprintf(stdout, "expected auth token expiry time: %s", ctime(&et.expires_on));
		fprintf(stdout, "actual auth token expiry time: %s", ctime(&t.expires_on));
		ASSERT_EQ(t.expires_on, et.expires_on);
		ASSERT_EQ(t.expires_in, et.expires_in);	
		ASSERT_EQ(t.access_token, et.access_token);	
		ASSERT_EQ(t.resource, et.resource);

	}

	// expires_on is a string dt in UTC format
	TEST_F(OAuthTokenTest, fromJSonDtUTCstring)
	{ 
		OAuthToken t;
		OAuthToken et = OAuthToken();
		et.access_token = "eyJ0eXAiOikIjoiYzg4";
		et.resource = "https://storage.azure.com/";
		et.token_type = "Bearer";
		time_t  exp = (time_t) 1586908151;
		et.expires_on = exp;

		std::string json_request_result = "{\"access_token\":\"eyJ0eXAiOikIjoiYzg4\",\"expires_on\":\"2020-04-14 16:49:11.72 +0000 UTC\",\"resource\":\"https://storage.azure.com/\",\"token_type\":\"Bearer\"}";
		json j;
		j = json::parse(json_request_result);
		from_json(j, t);
		
		fprintf(stdout, "expected auth token expiry time: %s", ctime(&et.expires_on));
		fprintf(stdout, "actual auth token expiry time: %s", ctime(&t.expires_on));
		ASSERT_EQ(t.expires_on, et.expires_on);
		ASSERT_EQ(t.access_token, et.access_token);	
		ASSERT_EQ(t.resource, et.resource);

	}

	// expires_on is a string dt in UTC format with a string month
	TEST_F(OAuthTokenTest, fromJSonDtAbbrMonthUTCstring)
	{ 
		OAuthToken t;
		OAuthToken et = OAuthToken();
		et.access_token = "eyJ0eXAiOikIjoiYzg4";
		et.resource = "https://storage.azure.com/";
		et.token_type = "Bearer";
		time_t  exp = (time_t) 1586908151;
		et.expires_on = exp;

		std::string json_request_result = "{\"access_token\":\"eyJ0eXAiOikIjoiYzg4\",\"expires_on\":\"2020-Apr-14 16:49:11.72 +0000 UTC\",\"resource\":\"https://storage.azure.com/\",\"token_type\":\"Bearer\"}";
		json j;
		j = json::parse(json_request_result);
		from_json(j, t);
		
		fprintf(stdout, "expected auth token expiry time: %s", ctime(&et.expires_on));
		fprintf(stdout, "actual auth token expiry time: %s", ctime(&t.expires_on));
		ASSERT_EQ(t.expires_on, et.expires_on);
		ASSERT_EQ(t.access_token, et.access_token);	
		ASSERT_EQ(t.resource, et.resource);

	}

}
