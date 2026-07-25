package httputil

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

func ParseUserID(c *gin.Context) (uuid.UUID, bool) {
    rawUserID, exists := c.Get("userID")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized context"})
        return uuid.Nil, false
    }
    userID, err := uuid.Parse(rawUserID.(string))
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to parse user ID"})
        return uuid.Nil, false
    }
    return userID, true
}