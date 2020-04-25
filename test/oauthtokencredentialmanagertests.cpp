#include "gtest/gtest.h"
#include "OAuthToken.h"
#include "OAuthTokenCredentialManager.h"
#include <time.h>
#include <sys/times.h>

#define CHECK_STRINGS(LEFTSTRING, RIGHTSTRING) ASSERT_EQ(0, LEFTSTRING.compare(RIGHTSTRING)) << "Strings failed equality comparison.  " << #LEFTSTRING << " is " << LEFTSTRING << ", " << #RIGHTSTRING << " is " << RIGHTSTRING << ".  "

// test class to check for OAuthToken credential manager operations.
namespace Tests
{
    class OAuthTokenCredentialManagerTest : public ::testing::Test {
    };

    // Test from json with all the parameters.
    TEST_F(OAuthTokenCredentialManagerTest, is_token_expired_forcurrentutc_success)
    { 
        OAuthToken et = OAuthToken();
        et.access_token = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSIsImtpZCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSJ9.eyJhdWQiOiJodHRwczovL3N0b3JhZ2UuYXp1cmUuY29tLyIsImlzcyI6Imh0dHBzOi8vc3RzLndpbmRvd3MubmV0LzcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0Ny8iLCJpYXQiOjE1ODY4MjE0NTEsIm5iZiI6MTU4NjgyMTQ1MSwiZXhwIjoxNTg2OTA4MTUxLCJhaW8iOiI0MmRnWURnWk1zSERvc3JZV1hyQ01uYnRsTm9UQUE9PSIsImFwcGlkIjoiYzg4MjI5Y2UtYzVlNC00OGJmLWI2NDktYzQ5MGM2ZThmMzQzIiwiYXBwaWRhY3IiOiIyIiwiaWRwIjoiaHR0cHM6Ly9zdHMud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3LyIsIm9pZCI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInN1YiI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInRpZCI6IjcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0NyIsInV0aSI6IjJORU5mZ2dzVTBxZ0JKMC1pQkZuQUEiLCJ2ZXIiOiIxLjAiLCJ4bXNfbWlyaWQiOiIvc3Vic2NyaXB0aW9ucy9iYTQ1YjIzMy1lMmVmLTQxNjktODgwOC00OWViMGQ4ZWJhMGQvcmVzb3VyY2Vncm91cHMvYmxvYmZ1c2UtcmcvcHJvdmlkZXJzL01pY3Jvc29mdC5Db21wdXRlL3ZpcnR1YWxNYWNoaW5lcy9uYXJhZGV2MTYwNCJ9.kBfu0O7sfiYXyRX-kBboyZ1MVM3bdWwKXGQh9N4BR6-xv7ut7HzaEatErRz_BfRDoQEMSwtaNAtrBG8rqWnQNgkAJxaN4vwtdipreOJVMF_94f4BQnxrsjCwuaiGsrZBWS3au2s2CayWSeHEc2vYE8edQBOY58FujLt6h5A1ePssE1TotveazLVesJ3bWgIfH2gedjiy8MKoPWB5GxstcrvuvzDXlG4lFDkUlI8LZ8s0Su8S7KzGPOdkb_1eAGhJdGtDSWI68FGYQsse8OYw0a-d-B06yW3i2NRlLE3_oCy4m-vBKtF2TtpJ5S1eYVa4SkDsl1sVLbJO7E_T4i4gpg";
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
        
        bool hasExpired  = is_token_expired_forcurrentutc(et);
        
        ASSERT_EQ(hasExpired, false);
    }
    
    // Test from json with all the parameters.
    TEST_F(OAuthTokenCredentialManagerTest, is_token_expired_forcurrentutc_failure)
    { 
        OAuthToken et = OAuthToken();
        et.access_token = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSIsImtpZCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSJ9.eyJhdWQiOiJodHRwczovL3N0b3JhZ2UuYXp1cmUuY29tLyIsImlzcyI6Imh0dHBzOi8vc3RzLndpbmRvd3MubmV0LzcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0Ny8iLCJpYXQiOjE1ODY4MjE0NTEsIm5iZiI6MTU4NjgyMTQ1MSwiZXhwIjoxNTg2OTA4MTUxLCJhaW8iOiI0MmRnWURnWk1zSERvc3JZV1hyQ01uYnRsTm9UQUE9PSIsImFwcGlkIjoiYzg4MjI5Y2UtYzVlNC00OGJmLWI2NDktYzQ5MGM2ZThmMzQzIiwiYXBwaWRhY3IiOiIyIiwiaWRwIjoiaHR0cHM6Ly9zdHMud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3LyIsIm9pZCI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInN1YiI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInRpZCI6IjcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0NyIsInV0aSI6IjJORU5mZ2dzVTBxZ0JKMC1pQkZuQUEiLCJ2ZXIiOiIxLjAiLCJ4bXNfbWlyaWQiOiIvc3Vic2NyaXB0aW9ucy9iYTQ1YjIzMy1lMmVmLTQxNjktODgwOC00OWViMGQ4ZWJhMGQvcmVzb3VyY2Vncm91cHMvYmxvYmZ1c2UtcmcvcHJvdmlkZXJzL01pY3Jvc29mdC5Db21wdXRlL3ZpcnR1YWxNYWNoaW5lcy9uYXJhZGV2MTYwNCJ9.kBfu0O7sfiYXyRX-kBboyZ1MVM3bdWwKXGQh9N4BR6-xv7ut7HzaEatErRz_BfRDoQEMSwtaNAtrBG8rqWnQNgkAJxaN4vwtdipreOJVMF_94f4BQnxrsjCwuaiGsrZBWS3au2s2CayWSeHEc2vYE8edQBOY58FujLt6h5A1ePssE1TotveazLVesJ3bWgIfH2gedjiy8MKoPWB5GxstcrvuvzDXlG4lFDkUlI8LZ8s0Su8S7KzGPOdkb_1eAGhJdGtDSWI68FGYQsse8OYw0a-d-B06yW3i2NRlLE3_oCy4m-vBKtF2TtpJ5S1eYVa4SkDsl1sVLbJO7E_T4i4gpg";
        et.expires_in = 4;
        et.not_before = 1586821451;
        et.resource = "https://storage.azure.com/";
        et.token_type = "Bearer";    
        time_t current_time;
        // get the current time        
        time(&current_time);
        
        //Get GMT time 
        
        struct tm *info;
       
        info = gmtime(&current_time );
        
        et.expires_on = mktime(info) + 4;
        
        bool hasExpired  = is_token_expired_forcurrentutc(et);
        
        ASSERT_EQ(hasExpired, true);
    }
    
    // Test from json with all the parameters.
    TEST_F(OAuthTokenCredentialManagerTest, is_token_expired_forcurrentutc_failure_expired)
    { 
        OAuthToken et = OAuthToken();
        et.access_token = "eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiIsIng1dCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSIsImtpZCI6IllNRUxIVDBndmIwbXhvU0RvWWZvbWpxZmpZVSJ9.eyJhdWQiOiJodHRwczovL3N0b3JhZ2UuYXp1cmUuY29tLyIsImlzcyI6Imh0dHBzOi8vc3RzLndpbmRvd3MubmV0LzcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0Ny8iLCJpYXQiOjE1ODY4MjE0NTEsIm5iZiI6MTU4NjgyMTQ1MSwiZXhwIjoxNTg2OTA4MTUxLCJhaW8iOiI0MmRnWURnWk1zSERvc3JZV1hyQ01uYnRsTm9UQUE9PSIsImFwcGlkIjoiYzg4MjI5Y2UtYzVlNC00OGJmLWI2NDktYzQ5MGM2ZThmMzQzIiwiYXBwaWRhY3IiOiIyIiwiaWRwIjoiaHR0cHM6Ly9zdHMud2luZG93cy5uZXQvNzJmOTg4YmYtODZmMS00MWFmLTkxYWItMmQ3Y2QwMTFkYjQ3LyIsIm9pZCI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInN1YiI6IjdmZmIyNTc0LTlmNmUtNDUyMC04MjkyLTYxNGZkZWZkOWZlMCIsInRpZCI6IjcyZjk4OGJmLTg2ZjEtNDFhZi05MWFiLTJkN2NkMDExZGI0NyIsInV0aSI6IjJORU5mZ2dzVTBxZ0JKMC1pQkZuQUEiLCJ2ZXIiOiIxLjAiLCJ4bXNfbWlyaWQiOiIvc3Vic2NyaXB0aW9ucy9iYTQ1YjIzMy1lMmVmLTQxNjktODgwOC00OWViMGQ4ZWJhMGQvcmVzb3VyY2Vncm91cHMvYmxvYmZ1c2UtcmcvcHJvdmlkZXJzL01pY3Jvc29mdC5Db21wdXRlL3ZpcnR1YWxNYWNoaW5lcy9uYXJhZGV2MTYwNCJ9.kBfu0O7sfiYXyRX-kBboyZ1MVM3bdWwKXGQh9N4BR6-xv7ut7HzaEatErRz_BfRDoQEMSwtaNAtrBG8rqWnQNgkAJxaN4vwtdipreOJVMF_94f4BQnxrsjCwuaiGsrZBWS3au2s2CayWSeHEc2vYE8edQBOY58FujLt6h5A1ePssE1TotveazLVesJ3bWgIfH2gedjiy8MKoPWB5GxstcrvuvzDXlG4lFDkUlI8LZ8s0Su8S7KzGPOdkb_1eAGhJdGtDSWI68FGYQsse8OYw0a-d-B06yW3i2NRlLE3_oCy4m-vBKtF2TtpJ5S1eYVa4SkDsl1sVLbJO7E_T4i4gpg";
        et.not_before = 1586821451;
        et.resource = "https://storage.azure.com/";
        et.token_type = "Bearer";    
        time_t current_time;
        // get the current time        
        time(&current_time);
        
        //Get GMT time 
        
        struct tm *info;
       
        info = gmtime(&current_time );
        
        et.expires_on = mktime(info) - 4;
        
        bool hasExpired  = is_token_expired_forcurrentutc(et);
        
        ASSERT_EQ(hasExpired, true);
    }
}
